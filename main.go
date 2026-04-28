package main

import (
	"fmt"
	"net/http"
	"os"
	"rinha-backend/src"
)

var ready = false

func main() {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /ready", func(w http.ResponseWriter, r *http.Request) {
		status := http.StatusOK

		if !ready {
			status = http.StatusServiceUnavailable
		}

		w.WriteHeader(status)
	})

	f, err := os.Open("dataset.bin")

	if err != nil {
		panic(err)
	}

	defer f.Close()

	ready = src.Mmap(f)

	if ready {
		mux.HandleFunc("POST /fraud-score", src.Fraudscore)
	}

	fmt.Println("Servidor rodando na porta 8080...")
	http.ListenAndServe(":8080", mux)
}
