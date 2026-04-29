package src

import (
	"encoding/json"
	"log"
	"net/http"
	"time"
)

const apiLogEnabled = true

func apiLog(format string, v ...interface{}) {
	if apiLogEnabled {
		log.Printf("[API] "+format, v...)
	}
}

func Fraudscore(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	apiLog("=> %s %s from %s", r.Method, r.URL.Path, r.RemoteAddr)

	var req Transaction

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apiLog("!! ERRO decode JSON: %v", err)
		http.Error(w, "JSON invalido", http.StatusBadRequest)
		return
	}

	if err := req.Validate(); err != nil {
		apiLog("!! ERRO validacao: %v", err)
		http.Error(w, err.Error(), http.StatusUnprocessableEntity)
		return
	}

	v := normalize(&req)

	// Mede o tempo do KNN scan, que eh a parte mais pesada.
	detectStart := time.Now()
	result := Detect(v)
	apiLog("   Detect() durou %v", time.Since(detectStart))

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(result)

	apiLog("<= 200 %s (total %v)", r.URL.Path, time.Since(start))
}
