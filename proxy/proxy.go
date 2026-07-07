package proxy

import (
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"time"
)

const (
	sessionHeader = "X-Session-ID"
	sessionCookie = "session_id"
)

type Proxy struct {
	router *Router
}

func NewProxy(router *Router) *Proxy {
	return &Proxy{router: router}
}

func (p *Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	balancer := p.router.Match(r.URL.Path)
	if balancer == nil {
		http.Error(w, "no route for path", http.StatusNotFound)
		return
	}

	var backend *Backend
	var err error

	if id := sessionID(r); id != "" {
		backend, err = balancer.NextForSession(id)
	} else {
		backend, err = balancer.Next()
	}
	if err != nil {
		http.Error(w, "service unavailable", http.StatusServiceUnavailable)
		return
	}
	p.newReverseProxy(backend).ServeHTTP(w, r)
}

// sessionID extracts a client's sticky-session key from the X-Session-ID
// header, falling back to the session_id cookie if the header is absent.
func sessionID(r *http.Request) string {
	if id := r.Header.Get(sessionHeader); id != "" {
		return id
	}
	if c, err := r.Cookie(sessionCookie); err == nil {
		return c.Value
	}
	return ""
}

func (p *Proxy) newReverseProxy(backend *Backend) *httputil.ReverseProxy {
	target := backend.URL
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

	// A response reaching ModifyResponse means the backend was reachable,
	// regardless of its status code, so it counts as a circuit breaker
	// success. ErrorHandler fires on transport-level failures and counts as a failure.
	rp.ModifyResponse = func(resp *http.Response) error {
		backend.RecordSuccess()
		return nil
	}

	rp.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		backend.RecordFailure()
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
