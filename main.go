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

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	go func() {
		ready = src.Mmap(f)
		if ready {
			mux.HandleFunc("POST /fraud-score", src.Fraudscore)
			fmt.Println("Dados carregados. API pronta para receber requisicoes.")
		} else {
			fmt.Println("FALHA ao carregar dataset. Endpoint /fraud-score nao registrado.")
		}
	}()

	fmt.Printf("Servidor escutando na porta %s...\n", port)
	if err := http.ListenAndServe(":"+port, mux); err != nil {
		panic(err)
	}
}
