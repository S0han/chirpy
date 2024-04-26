package main

import (
	"log"
	"encoding/json"
	"net/http"
	"fmt"
	"sync/atomic"
	"strings"
)

func main() {

	const port = "8080"

	//Create an empty serve mux
	mux := http.NewServeMux()
	corsMux := middlewareCors(mux)
	server := &http.Server {
		Addr: ":" + port,
		Handler: corsMux,
	}

	mux.HandleFunc("/api/healthz", healthzHandler)

	handleState :=  apiCfg{}

	mux.HandleFunc("/admin/metrics", func(w http.ResponseWriter, r *http.Request) {
		handleState.processedRequests(w, r)
	})

	mux.HandleFunc("/api/reset", func(w http.ResponseWriter, r *http.Request) {
		handleState.resetHits(w, r)
	})

	mux.HandleFunc("/api/validate_chirp", validChirpHandler)

	mux.Handle("/app/", handleState.middlewareMetricsInc(http.StripPrefix("/app/", http.FileServer(http.Dir("app")))))

	log.Printf("Serving on port: %s\n", port)
	log.Fatal(server.ListenAndServe())
}

func validChirpHandler(w http.ResponseWriter, r *http.Request) {	
	
	if r.Method != http.MethodPost {
		http.Error(w, "Only POST is allowed", http.StatusMethodNotAllowed)
		return
	}

	//decode json
	type parameters struct {
		Body string `json:"body"`
	}

	decoder := json.NewDecoder(r.Body)
	p := parameters{}
	err := decoder.Decode(&p)
	if err != nil {
		http.Error(w, `{"error": "Something went wrong"}`, http.StatusMethodNotAllowed)
		w.WriteHeader(400)
		return
	}

	if len(p.Body) > 140 {
		http.Error(w, `{"error": "Chirp is too long"}`, http.StatusBadRequest)
		w.WriteHeader(400)
		return
	}

	response := map[string]bool {"valid": true}
	responseJSON, err := json.Marshal(response)
	if err != nil {
		http.Error(w, `{"error": "Something went wrong"}`, http.StatusBadRequest)
		w.WriteHeader(400)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(200)
	w.Write(responseJSON)
}

type apiCfg struct {
	fileserverHits int64
}

func (cfg *apiCfg) middlewareMetricsInc(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&cfg.fileserverHits, 1)
		next.ServeHTTP(w, r)
	})
}

func (cfg *apiCfg) processedRequests(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Only GET is allowed", http.StatusMethodNotAllowed)
		return
	}
	currentHits := atomic.LoadInt64(&cfg.fileserverHits)
	hits := fmt.Sprintf(`
	<html>
	<body>
		<h1>Welcome, Chirpy Admin</h1>
		<p>Chirpy has been visited %d times!</p>
	</body>
	
	</html>
	`, currentHits)
	w.Header().Set("Content-Type", "text/html")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(hits))
}

func (cfg *apiCfg) resetHits(w http.ResponseWriter, r *http.Request) {
	atomic.StoreInt64(&cfg.fileserverHits, 0)
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Hits reset to 0"))
}

func healthzHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

func middlewareCors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS, PUT, DELETE")
		w.Header().Set("Access-Control-Allow-Headers", "*")
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}
		next.ServeHTTP(w, r)
	})
}