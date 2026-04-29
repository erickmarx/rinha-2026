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

	var req Transaction

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		// Se o cliente cancelou a requisicao (timeout, connection reset),
		// o body vem incompleto e o decoder retorna EOF.
		if r.Context().Err() != nil {
			apiLog("WARN cliente cancelou: %v", r.Context().Err())
		} else {
			apiLog("ERRO decode JSON: %v", err)
		}
		http.Error(w, "JSON invalido", http.StatusBadRequest)
		return
	}

	if err := req.Validate(); err != nil {
		apiLog("ERRO validacao: %v", err)
		http.Error(w, err.Error(), http.StatusUnprocessableEntity)
		return
	}

	v := normalize(&req)

	// Mede o tempo do KNN scan. Se demorar mais que 50ms,
	// loga como WARN — pode ser causa de timeout.
	detectStart := time.Now()
	result := Detect(v)
	detectDuration := time.Since(detectStart)
	if detectDuration > 50*time.Millisecond {
		apiLog("WARN Detect() lento: %v", detectDuration)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(result)

	// Só loga se o total da requisicao foi lento (> 100ms).
	if total := time.Since(start); total > 100*time.Millisecond {
		apiLog("WARN requisicao lenta: %v", total)
	}
}
