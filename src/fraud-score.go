package src

import (
	"encoding/json"
	"log"
	"net/http"
	"time"
)

const apiLogEnabled = false

func apiLog(format string, v ...interface{}) {
	if apiLogEnabled {
		log.Printf("[API] "+format, v...)
	}
}

// fraudScoreResponses pré-computa as 6 respostas JSON possíveis (k=5, threshold=0.60).
// Elimina completamente a serialização JSON no hot path.
var fraudScoreResponses = [6][]byte{
	[]byte(`{"approved":true,"fraud_score":0}`),
	[]byte(`{"approved":true,"fraud_score":0.2}`),
	[]byte(`{"approved":true,"fraud_score":0.4}`),
	[]byte(`{"approved":false,"fraud_score":0.6}`),
	[]byte(`{"approved":false,"fraud_score":0.8}`),
	[]byte(`{"approved":false,"fraud_score":1}`),
}

func Fraudscore(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	var req Transaction

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
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

	detectStart := time.Now()
	fraudCount := Detect(v)
	detectDuration := time.Since(detectStart)
	if detectDuration > 50*time.Millisecond {
		apiLog("WARN Detect() lento: %v", detectDuration)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(fraudScoreResponses[fraudCount])

	if total := time.Since(start); total > 100*time.Millisecond {
		apiLog("WARN requisicao lenta: %v", total)
	}
}
