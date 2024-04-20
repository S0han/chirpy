package main

import (
	"log"
	"net/http"
	"fmt"
	"sync/atomic"
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

	mux.HandleFunc("POST /api/validate_chirp", validChirpHandler)

	mux.Handle("/app/", handleState.middlewareMetricsInc(http.StripPrefix("/app/", http.FileServer(http.Dir("app")))))

	log.Printf("Serving on port: %s\n", port)
	log.Fatal(server.ListenAndServe())
}

func validChirpHandler(w http.ResponseWriter, r *http.Request) {	
	
	//decode json
	type parameters struct {
		Body string `json:"body"`
		Error string `json:"error"`
	}

	decoder := json.NewDecoder(r.Body)
	params := parameters{}
	err := decoder.Decode(&params)
	if err != nil {
		log.Printf("Something went wrong")
		w.WriteHeader(500)
		return
	}

	//encode json
	type returnVals struct {
		CreatedAt time.Time `json:"created_at"`
		ID int `json:"id"`
	}
	respBody := returnVals{
		CreatedAt: time.Now(),
		ID: 123,
	}
	dat, err := json.Marshal(respBody)
	if err !=  nil {
		if len(dat) > 140 {
			log.Printf("Chirp is too long")
		}
	}
	w.Header().Set("Content-Type", "application/json")
	log.Printf("true")
	w.writeHeader(200)
	w.Write(dat)
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