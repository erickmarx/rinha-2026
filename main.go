package main

import (
	"fmt"
	"net"
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

	udsPath := os.Getenv("UDS_PATH")
	var listener net.Listener
	if udsPath != "" {
		os.Remove(udsPath)
		l, err := net.Listen("unix", udsPath)
		if err != nil {
			panic(err)
		}
		listener = l
		os.Chmod(udsPath, 0666)
		fmt.Printf("Servidor escutando em Unix socket %s...\n", udsPath)
	} else {
		port := os.Getenv("PORT")
		if port == "" {
			port = "8080"
		}
		l, err := net.Listen("tcp", ":"+port)
		if err != nil {
			panic(err)
		}
		listener = l
		fmt.Printf("Servidor escutando na porta %s...\n", port)
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

	if err := http.Serve(listener, mux); err != nil {
		panic(err)
	}
}
