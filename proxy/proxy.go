package proxy

import (
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"
)

type Proxy struct {
	balancer *Balancer
}

func NewProxy(balancer *Balancer) *Proxy {
	return &Proxy{balancer: balancer}
}

func (p *Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	backend, err := p.balancer.Next()
	if err != nil {
		http.Error(w, "service unavailable", http.StatusServiceUnavailable)
		return
	}
	p.newReverseProxy(backend.URL).ServeHTTP(w, r)
}

func (p *Proxy) newReverseProxy(target *url.URL) *httputil.ReverseProxy {
	rp := httputil.NewSingleHostReverseProxy(target)

	rp.Transport = &http.Transport{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 20,
		IdleConnTimeout:     90 * time.Second,
		DisableCompression:  false,
	}

	originalDirector := rp.Director
	rp.Director = func(req *http.Request) {
		originalDirector(req)
		req.Header.Set("X-Forwarded-Host", req.Host)
		req.Header.Set("X-Proxy", "goproxy/1.0")
		req.Host = target.Host
	}

	rp.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		log.Printf("backend %s error: %v", target, err)
		http.Error(w, fmt.Sprintf("bad gateway: %v", err), http.StatusBadGateway)
	}

	return rp
}

func StartHealthChecks(balancer *Balancer, interval time.Duration) {
	for _, b := range balancer.Backends() {
		b.StartHealthCheck(interval)
	}
	log.Printf("health checks started (interval=%s)", interval)
}
