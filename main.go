package main

import (
	"log"
	"net/http"
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

	mux.HandleFunc("/healthz", healthzHandler)

	mux.Handle("/app/", http.StripPrefix("/app/", http.FileServer(http.Dir("."))))

	log.Printf("Serving on port: %s\n", port)
	log.Fatal(server.ListenAndServe())
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