package main

import (
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

var backends []*url.URL

var idx atomic.Uint64

// bytePool implementa a interface httputil.BufferPool.
// httputil.ReverseProxy exige Get() []byte e Put([]byte), mas sync.Pool
// trabalha com interface{} (any). Esse wrapper faz a conversao de tipos.
type bytePool struct {
	pool *sync.Pool
}

func (p *bytePool) Get() []byte {
	return p.pool.Get().([]byte)
}

func (p *bytePool) Put(b []byte) {
	p.pool.Put(b)
}

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
	// TRANSPORT CUSTOMIZADO
	// =========================================================================
	// O ReverseProxy usa http.DefaultTransport por padrao. O problema:
	// - MaxIdleConnsPerHost PADRAO eh 2. Isso significa que entre o LB e CADA API
	//   so existem 2 conexoes TCP reutilizaveis. Se o k6 enviar 50 reqs paralelas,
	//   o LB fica abrindo e fechando conexoes o tempo todo (TCP handshake + TLS
	//   se houvesse). Com 1 CPU, esse overhead mata a performance.
	// - DisableCompression: true evita que o LB gaste CPU descomprimindo respostas
	//   gzip. Como controlamos ambos os lados, nao precisamos de compressao.
	transport := &http.Transport{
		MaxIdleConns:        100,          // total de conexoes idle no pool
		MaxIdleConnsPerHost: 100,          // conexoes idle POR API (era 2!)
		IdleConnTimeout:     90 * time.Second,
		DisableCompression:  true,         // economiza CPU
		DisableKeepAlives:   false,        // explicita: keep-alive ligado
	}

	// =========================================================================
	// BUFFER POOL
	// =========================================================================
	// O ReverseProxy precisa de buffers temporarios para ler/escrever dados
	// durante o proxy. Sem BufferPool, ele faz make([]byte, 32*1024) a CADA
	// requisicao. Com alta carga isso gera pressao no garbage collector.
	//
	// httputil.BufferPool eh uma interface que exige Get() []byte e Put([]byte).
	// sync.Pool sozinho nao satisfaz essa interface (ele usa Get() any).
	// Por isso criamos um wrapper tipado.
	bufferPool := &bytePool{
		pool: &sync.Pool{
			New: func() any {
				return make([]byte, 32*1024) // 32KB = tamanho padrao do ReverseProxy
			},
		},
	}

	proxy := httputil.ReverseProxy{
		Transport:  transport,
		BufferPool: bufferPool,

		Director: func(req *http.Request) {
			// Round robin atômico.
			i := int(idx.Add(1)-1) % len(backends)
			target := backends[i]

			req.URL.Scheme = target.Scheme
			req.URL.Host = target.Host
			req.Host = target.Host

			if logsEnabled {
				log.Printf("[LB] => %s %s -> %s", req.Method, req.URL.Path, target.Host)
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
