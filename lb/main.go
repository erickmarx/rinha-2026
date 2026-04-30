package main

import (
	"context"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
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

var proxies []*httputil.ReverseProxy
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
			transport := &http.Transport{
				DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
					return net.Dial("unix", path)
				},
				MaxIdleConns:        100,
				MaxIdleConnsPerHost: 100,
				DisableCompression:  true,
			}
			proxy := &httputil.ReverseProxy{
				Transport: transport,
				Director: func(req *http.Request) {
					req.URL.Scheme = "http"
					req.URL.Host = "localhost"
					req.Host = "localhost"
				},
				ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
					lbLog("ERRO UDS %s %s: %v", r.Method, r.URL.Path, err)
					http.Error(w, "Bad Gateway", http.StatusBadGateway)
				},
			}
			proxies = append(proxies, proxy)
		} else {
			u, err := url.Parse(s)
			if err != nil {
				panic(err)
			}
			proxy := httputil.NewSingleHostReverseProxy(u)
			proxy.Transport = &http.Transport{
				MaxIdleConns:        100,
				MaxIdleConnsPerHost: 100,
				DisableCompression:  true,
			}
			proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
				lbLog("ERRO TCP %s %s -> %s: %v", r.Method, r.URL.Path, u.Host, err)
				http.Error(w, "Bad Gateway", http.StatusBadGateway)
			}
			proxies = append(proxies, proxy)
		}
	}

	lbLog("Iniciado com %d backend(s): %v", len(proxies), raw)

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		i := int(idx.Add(1)-1) % len(proxies)
		proxies[i].ServeHTTP(w, r)
	})

	lbLog("Escutando na porta 8080")
	http.ListenAndServe(":8080", nil)
}
