package main

import (
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"
	"sync/atomic"
)

// backends armazena a lista de URLs das APIs para onde o LB vai distribuir as requisições.
// O tipo *url.URL já vem parseado (scheme, host, path, etc.).
var backends []*url.URL

// idx é um contador atômico usado pelo round robin.
// Usamos atomic.Uint64 para evitar race conditions quando múltiplas goroutines
// (uma por requisição HTTP) incrementam o contador ao mesmo tempo.
var idx atomic.Uint64

func main() {
	// BACKENDS vem da variável de ambiente, definida no docker-compose.
	// Formato esperado: "http://api1:8080,http://api2:8080"
	// O valor padrão serve para testes locais fora do Docker.
	raw := os.Getenv("BACKENDS")
	if raw == "" {
		raw = "http://api1:8080,http://api2:8080"
	}

	// Quebra a string em várias URLs usando vírgula como delimitador.
	// Cada URL válida é convertida para *url.URL e adicionada ao slice backends.
	for _, s := range strings.Split(raw, ",") {
		u, err := url.Parse(s)
		if err != nil {
			log.Fatal(err)
		}
		backends = append(backends, u)
	}

	// httputil.ReverseProxy é o coração do load balancer.
	// Ele recebe a requisição do cliente, encaminha para o backend escolhido
	// e devolve a resposta do backend de volta ao cliente, sem modificar nada.
	proxy := httputil.ReverseProxy{
		// Director é executado para CADA requisição antes de enviá-la ao backend.
		// Aqui aplicamos o round robin: incrementamos o contador atômico e usamos
		// o módulo (%) para circular entre os backends disponíveis.
		Director: func(req *http.Request) {
			// Add(1) retorna o valor APÓS o incremento. Subtraímos 1 para usar base 0.
			i := int(idx.Add(1)-1) % len(backends)
			target := backends[i]

			// Apontamos a requisição para o backend escolhido.
			// Scheme e Host da URL definem para onde o HTTP client interno vai conectar.
			req.URL.Scheme = target.Scheme
			req.URL.Host = target.Host
			req.Host = target.Host
		},
		// ModifyResponse e ErrorHandler ficam nil de propósito:
		// não queremos interceptar, transformar ou tomar decisões sobre a resposta.
		// O proxy repassa exatamente o que o backend respondeu.
		ModifyResponse: nil,
		ErrorHandler:   nil,
	}

	fmt.Println("LB listening on :8080")
	// Inicia o servidor HTTP na porta 8080 usando nosso proxy como handler.
	// Todas as requisições que chegam aqui são encaminhadas para as APIs.
	log.Fatal(http.ListenAndServe(":8080", &proxy))
}
