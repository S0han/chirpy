package main

import (
	"os"
	"log"
	"encoding/json"
	"net/http"
	"fmt"
	"sync"
	"sync/atomic"
	"strings"
	"sort"
	"strconv"
)

func main() {

	const port = "8080"

	//initialize a new db
	db, err := NewDB("database.json")
	if err != nil {
		log.Fatalf("failed to initialize database: %v", err)
	}

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

	mux.HandleFunc("/api/chirps", chirpHandler(db))

	mux.HandleFunc("/api/chirps/", getChirpById(db))

	mux.Handle("/app/", handleState.middlewareMetricsInc(http.StripPrefix("/app/", http.FileServer(http.Dir("app")))))

	log.Printf("Serving on port: %s\n", port)
	log.Fatal(server.ListenAndServe())
}

func getChirpById(db *DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		urlParts := strings.Split(r.URL.Path, "/")
		chirpIDStr := urlParts[len(urlParts)-1]

		chirpID, err := strconv.Atoi(chirpIDStr)
		if err != nil {
			http.Error(w, "Invalid chirp ID", http.StatusBadRequest)
			return
		}

		chirp, exists, err := db.GetChirpByID(chirpID)
		if err != nil {
			http.Error(w, "chirp not found", http.StatusNotFound)
			return
		}

		if !exists {
			http.Error(w, "Chirp not found", http.StatusNotFound)
			return
		}

		response := map[string]interface{}{
			"id": chirp.Id,
			"body": chirp.Body,
		}

		responseJSON, err := json.Marshal(response)
		if err != nil {
			http.Error(w, "Failed to encode response", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(responseJSON)
	}
}

func chirpHandler(db *DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			allChirps, err := db.GetChirps()
			if err != nil {
				respondWithError(w, http.StatusInternalServerError, `{"error": "Failed to get chirps"}`)
				return
			}
			respondWithJSON(w, http.StatusOK, allChirps)

		case http.MethodPost:
			_, data, err := validChirpHandler(r)
			if err != nil {
				respondWithError(w, http.StatusBadRequest, `{"error": "Something went wrong"}`)
				return
			}

			chirp, err := db.CreateChirp(data["body"])
			if err != nil {
				respondWithError(w, http.StatusInternalServerError, `{"error": "Something went wrong"}`)
				return
			}
			respondWithJSON(w, http.StatusCreated, chirp)
		default:
			respondWithError(w, http.StatusMethodNotAllowed, `{"error": "Method not allowed"}`)
		}
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
	db := &DB{
		path: path,
		mux: &sync.RWMutex{},
	}

	err := ensureDB(path)
	if err != nil {
		return nil, err
	}

	return db, nil
}

func (db *DB) CreateChirp(body string) (Chirp, error) {
	db.mux.Lock()
	defer db.mux.Unlock()
	
	data, err := os.ReadFile(db.path)
	if err != nil {
		return Chirp{}, err
	}

	var chirpHolder = new(DBStructure)

	if err := json.Unmarshal(data, &chirpHolder); err != nil {
		return Chirp{}, err
	}

	maxVal := 0
	for _, val := range chirpHolder.Chirps {
		if val.Id > maxVal {
			maxVal = val.Id
		}
	}

	nextID := maxVal + 1
	newChirp := Chirp {
		Id: nextID,
		Body: body,
	}

	chirpHolder.Chirps[nextID] = newChirp

	newMap, err := json.Marshal(chirpHolder)
	if err != nil {
		return Chirp{}, err
	}

	if err = os.WriteFile(db.path, newMap, 0644); err != nil {
		return Chirp{}, err
	}

	return newChirp, nil
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

	sort.Slice(chirpSlice, func(i, j int) bool {
		return chirpSlice[i].Id < chirpSlice[j].Id
	})

	return chirpSlice, nil
}

func (db *DB) GetChirpByID(id int) (Chirp, bool, error) {
	db.mux.RLock()
	defer db.mux.RUnlock()

	data, err := os.ReadFile(db.path)
	if err != nil {
		return Chirp{}, false, err
	}

	var chirpHolder = new(DBStructure)
	if err := json.Unmarshal(data, &chirpHolder); err != nil {
		return Chirp{}, false, err
	}

	chirp, exists := chirpHolder.Chirps[id]
	return chirp, exists, nil
}

func ensureDB(path string) error {
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			file, createErr := os.Create(path)
			if createErr != nil {
				return createErr
			}
			defer file.Close()

			initialData := []byte(`{"chirps":{}}`)
			if err := os.WriteFile(path, initialData, 0644); err != nil {
				return err
			}
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

func validChirpHandler(r *http.Request) (bool, map[string]string, error) {	
	var body map[string]string
	err := json.NewDecoder(r.Body).Decode(&body)
	if err != nil {
		return false, nil, err
	}

	chirpBody := body["body"]
	if chirpBody == "" {
		return false, nil, fmt.Errorf("chirp body is required")
	}

	if len(chirpBody) > 140 {
		return false, nil, fmt.Errorf("chirp is too long")
	}

	cleaned_body := removeProfanity(chirpBody)
	response := map[string]string {"body": cleaned_body}

	return true, response, nil
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
	w.Header().Set("Content-Type", "application/json")
	http.Error(w, msg, code)
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
	
	</html>`, currentHits)
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