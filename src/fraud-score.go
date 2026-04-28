package src

import (
	"encoding/json"
	"net/http"
)

func Fraudscore(w http.ResponseWriter, r *http.Request) {
	var req Transaction

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "JSON inválido", http.StatusBadRequest)
		return
	}

	if err := req.Validate(); err != nil {
		http.Error(w, err.Error(), http.StatusUnprocessableEntity)
		return
	}

	v := normalize(&req)

	w.Header().Set("Content-Type", "application/json")

	w.WriteHeader(http.StatusOK)

	json.NewEncoder(w).Encode(Detect(v))
}
