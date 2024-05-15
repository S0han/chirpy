package main

import (
	"os"
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

	mux.HandleFunc("/api/chirps", chirpHandler)

	mux.Handle("/app/", handleState.middlewareMetricsInc(http.StripPrefix("/app/", http.FileServer(http.Dir("app")))))

	log.Printf("Serving on port: %s\n", port)
	log.Fatal(server.ListenAndServe())
}

func chirpHandler(w http.ResponseWriter, r *http.Request) {



	method := r.Method
	switch method {
		case http.MethodGet:
			allChirps, err := db.GetChirps()
			if err != nil || !isValid {
				respondWithError(w, http.StatusInternalServerError, `{"error": "Failed to get chirps"}`)
			}
			respondWithJSON(w, http.StatusOK, allChirps)
		case http.MethodPost:
			data, err := validChirpHandler(r)
			if err != nil {
				respondWithError(w, 400, `{"error": "Something went wrong"}`)
			}
			chirp, err := CreateChirp(data.body)
			if err != nil {
				respondWithError(w, 400, `{"error": "Something went wrong"}`)
			}
			respondWithJSON(w, http.StatusOK, chirp)
		default:
			respondWithError(w, http.StatusMethodNotAllowed, `{"error": "Method not allowed"}`)
	}

}

type Chirp struct {
	Id int  `json:"id"`
	Body string `json:"body"`
}

type DB struct {
	path string 
	mux *sync.RWMutex
}

type DBStructure struct {
	Chirps map[int]Chirp `json:"chirps"`
}

func NewDB(path string) (*DB, error) {
	
	_, err := ensureDB(path)
	if err != nil {
		return nil, err
	}

	db := &DB{
		path: path,
		mux: &sync.RWMutex{},
	}

	return db, nil
}

func (db *DB) CreateChirp(body string) (Chirp, error) {
	
	data, err := os.ReadFile(db.path)
	if err != nil {
		return Chirp{}, err
	}

	var chirpHolder = new(DBStructure)

	if err := json.Unmarshal(data, &chirpHolder); err != nil {
		return Chirp{}, err
	}

	maxVal := 0

	for _, val := range(chirpHolder.Chirps) {

		if val.Id > maxVal {
			maxVal = val.Id
		}
	}

	maxVal++

	newChirp := Chirp {
		Id: maxVal,
		Body: body,
	}

	chirpHolder.Chirps[maxVal] = newChirp

	newMap, err := json.Marshal(chirpHolder)
	if err != nil {
		return Chirp{}, err
	}

	err = os.WriteFile(db.path, newMap, os.ModePerm)
	if err != nil {
		return Chirp{}, err
	}

	return newChirp, err
}

func (db *DB) GetChirps() ([]Chirp, error) {
	data, err := os.ReadFile(db.path)
	if err != nil {
		return []Chirp{}, err
	}

	var chirpHolder = new(DBStructure)
	chirpSlice := []Chirp{}

	if err := json.Unmarshal(data, &chirpHolder); err != nil {
		return []Chirp{}, err
	}

	for _, val := range(chirpHolder.Chirps) {
		chirpSlice = append(chirpSlice, val)
	}

	return chirpSlice, err
}

func (db *DB) ensureDB() error {

	if _, err := os.Stat(db.path); err != nil {
		if os.IsNotExist(err) {
			file, createErr := os.Create(db.path)
			if createErr != nil {
				return createErr
			}
			file.Close()
		} else {
			return err
		}
	}

	return nil
}

func (db *DB) loadDB() (DBStructure, error) {

	data, err := os.ReadFile(db.path)
	if err != nil {
		return DBStructure{}, err
	}

	var loadData = new(DBStructure)

	if err := json.Unmarshal(data, &loadData); err != nil {
		return DBStructure{}, err
	}

	return *loadData, nil
}

func (db *DB) writeDB(dbStructure DBStructure) error {

	data, err := json.Marshal(dbStructure)
	if err != nil {
		return err
	}

	err = os.WriteFile(db.path, data, os.ModePerm)
	if err != nil {
		return err
	}

	return nil
}

func validChirpHandler(r *http.Request) (bool, error) {	

	decoder := json.NewDecoder(r.Body)
	p := Chirp{}
	err := decoder.Decode(&p)
	if err != nil {
		return false, err
	}

	if len(p.Body) > 140 {
		return false, fmt.Errorf("chirp is too long")
	}

	cleaned_body := removeProfanity(p.Body)

	//change this to body form cleaned_body to satisfy the new requirements
	response := map[string]string {"body": cleaned_body}

	return true, nil
}

func respondWithJSON(w http.ResponseWriter, code int, payload interface{}) {
	responseJSON, err := json.Marshal(payload)
	if err != nil {
		respondWithError(w, 400, `{"error": "Something went wrong"}`)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	w.Write(responseJSON)
}

func respondWithError(w http.ResponseWriter, code int, msg string) {
	http.Error(w, msg, http.StatusBadRequest)
	w.WriteHeader(code)
	return
}

func removeProfanity(chirp string) string {
	split := strings.Split(chirp, " ")

	badWords := map[string]bool{
		"kerfuffle": true,
		"sharbert": true,
		"fornax": true,
	}

	for i, word := range split {
		
		lowerWord := strings.ToLower(word)

		if badWords[lowerWord] {
			split[i] = "****"
		} 
	}

	filtered := strings.Join(split, " ")
	return filtered
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