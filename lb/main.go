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

// lbLogEnabled controla se o LB imprime logs.
// Em producao, mantenha false. Em desenvolvimento, true para debugar.
// Como eh constante, o compilador Go elimina o codigo morto quando false.
const lbLogEnabled = false

func lbLog(format string, v ...interface{}) {
	if lbLogEnabled {
		log.Printf("[LB] "+format, v...)
	}
}

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
			panic(err)
		}
		backends = append(backends, u)
	}

	lbLog("Iniciado com %d backend(s): %v", len(backends), raw)

	// Transport com keep-alive reabilitado. Sem keep-alive, cada requisicao
	// abre uma nova conexao TCP (handshake + FIN), o que mata a CPU do LB
	// em alta carga. Com keep-alive, conexoes sao reutilizadas.
	transport := &http.Transport{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 100,
		DisableCompression:  true,
	}

	proxy := httputil.ReverseProxy{
		Transport: transport,
		Director: func(req *http.Request) {
			i := int(idx.Add(1)-1) % len(backends)
			target := backends[i]
			req.URL.Scheme = target.Scheme
			req.URL.Host = target.Host
			req.Host = target.Host
		},
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			lbLog("ERRO ao encaminhar %s %s -> %s: %v", r.Method, r.URL.Path, r.Host, err)
			http.Error(w, "Bad Gateway", http.StatusBadGateway)
		},
	}

	lbLog("Escutando na porta 8080")
	http.ListenAndServe(":8080", &proxy)
}
