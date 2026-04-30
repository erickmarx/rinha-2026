package main

import (
	"context"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync/atomic"
)

const lbLogEnabled = false

func lbLog(format string, v ...interface{}) {
	if lbLogEnabled {
		log.Printf("[LB] "+format, v...)
	}
}

type backend struct {
	target *url.URL
	client *http.Client
}

var backends []backend
var idx atomic.Uint64

func main() {
	raw := os.Getenv("BACKENDS")
	if raw == "" {
		raw = "http://api1:8080,http://api2:8080"
	}

	for _, s := range strings.Split(raw, ",") {
		s = strings.TrimSpace(s)
		if strings.HasPrefix(s, "unix://") {
			path := strings.TrimPrefix(s, "unix://")
			client := &http.Client{
				Transport: &http.Transport{
					DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
						return net.Dial("unix", path)
					},
					MaxIdleConns:        100,
					MaxIdleConnsPerHost: 100,
					DisableCompression:  true,
				},
			}
			backends = append(backends, backend{
				target: &url.URL{Scheme: "http", Host: "localhost"},
				client: client,
			})
		} else {
			u, err := url.Parse(s)
			if err != nil {
				panic(err)
			}
			client := &http.Client{
				Transport: &http.Transport{
					MaxIdleConns:        100,
					MaxIdleConnsPerHost: 100,
					DisableCompression:  true,
				},
			}
			backends = append(backends, backend{
				target: u,
				client: client,
			})
		}
	}

	lbLog("Iniciado com %d backend(s): %v", len(backends), raw)

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		i := int(idx.Add(1)-1) % len(backends)
		b := backends[i]

		req := new(http.Request)
		*req = *r
		req.URL = b.target.ResolveReference(r.URL)
		req.RequestURI = ""
		req.Host = b.target.Host

		resp, err := b.client.Do(req)
		if err != nil {
			lbLog("ERRO ao encaminhar %s %s -> %s: %v", r.Method, r.URL.Path, b.target.Host, err)
			http.Error(w, "Bad Gateway", http.StatusBadGateway)
			return
		}
		defer resp.Body.Close()

		for k, vv := range resp.Header {
			for _, v := range vv {
				w.Header().Add(k, v)
			}
		}
		w.WriteHeader(resp.StatusCode)
		io.Copy(w, resp.Body)
	})

	lbLog("Escutando na porta 8080")
	http.ListenAndServe(":8080", nil)
}
