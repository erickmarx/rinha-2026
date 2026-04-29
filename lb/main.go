package main

import (
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"
	"sync/atomic"
)

var backends []*url.URL

var idx atomic.Uint64

func main() {
	raw := os.Getenv("BACKENDS")
	if raw == "" {
	raw = "http://api1:8080,http://api2:8080"
	}

	for _, s := range strings.Split(raw, ",") {
		u, err := url.Parse(s)
		if err != nil {
			log.Fatal(err)
		}
		backends = append(backends, u)
	}

	// LB_LOG habilita/desabilita logs de requisicao.
	// Em producao com 1 CPU, logs consomem ciclos preciosos (mutex do logger,
	// formatacao de strings, syscall de escrita). Desligar aumenta throughput.
	logsEnabled := true
	log.Printf("[LB] Iniciado com %d backend(s): %v | logs=%v", len(backends), raw, logsEnabled)

	// =========================================================================
	// TRANSPORT
	// =========================================================================
	// DisableKeepAlives: true = cada requisicao abre uma nova conexao TCP.
	// Isso elimina o problema de desalinhamento de body em conexoes reutilizadas
	// (keep-alive), que causa "unexpected EOF" no JSON decoder da API em alta
	// carga. O overhead de TCP handshake eh aceitavel para JSON pequeno.
	transport := &http.Transport{
		DisableKeepAlives: true,
	}

	proxy := httputil.ReverseProxy{
		Transport: transport,

		Director: func(req *http.Request) {
			// Round robin atômico.
			i := int(idx.Add(1)-1) % len(backends)
			target := backends[i]

			req.URL.Scheme = target.Scheme
			req.URL.Host = target.Host
			req.Host = target.Host

			if logsEnabled {
				log.Printf("[LB] => %s %s -> %s (len=%d)", req.Method, req.URL.Path, target.Host, req.ContentLength)
			}
		},

		ModifyResponse: func(resp *http.Response) error {
			if logsEnabled {
				log.Printf("[LB] <= %d %s <- %s", resp.StatusCode, resp.Request.URL.Path, resp.Request.Host)
			}
			return nil
		},

		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			if logsEnabled {
				log.Printf("[LB] !! ERRO ao encaminhar %s %s -> %s: %v", r.Method, r.URL.Path, r.Host, err)
			}
			// Devolve 502 imediatamente para o cliente nao ficar pendurado.
			http.Error(w, "Bad Gateway", http.StatusBadGateway)
		},
	}

	log.Println("LB listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", &proxy))
}
