package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"os"

	"github.com/S0han/chirpy/webhooks/database"
)

func (cfg *apiConfig) handlerWebhook(w http.ResponseWriter, r *http.Request) {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	const prefix = "ApiKey "
	if !strings.HasPrefix(authHeader, prefix) {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	apiKey := strings.TrimPrefix(authHeader, prefix)

	expectedApiKey := os.Getenv("API_KEY")
	if apiKey != expectedApiKey {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	type parameters struct {
		Event string `json:"event"`
		Data  struct {
			UserID int `json:"user_id"`
		}
	}

	decoder := json.NewDecoder(r.Body)
	params := parameters{}
	err := decoder.Decode(&params)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't decode parameters")
		return
	}

	if params.Event != "user.upgraded" {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	_, err = cfg.DB.UpgradeChirpyRed(params.Data.UserID)
	if err != nil {
		if errors.Is(err, database.ErrNotExist) {
			respondWithError(w, http.StatusNotFound, "Couldn't find user")
			return
		}
		respondWithError(w, http.StatusInternalServerError, "Couldn't update user")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}