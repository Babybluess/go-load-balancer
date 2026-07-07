package proxy

import (
	"log"
	"net/http"
	"net/url"
	"sync/atomic"
	"time"
)

type Backend struct {
	URL    *url.URL
	Weight int
	alive  atomic.Bool
}

func NewBackend(rawURL string) (*Backend, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, err
	}
	b := &Backend{URL: u, Weight: 1}
	b.alive.Store(true)
	return b, nil
}

func (b *Backend) IsAlive() bool   { return b.alive.Load() }
func (b *Backend) SetAlive(v bool) { b.alive.Store(v) }

func (b *Backend) StartHealthCheck(interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for range ticker.C {
			b.checkOnce()
		}
	}()
}

func (b *Backend) checkOnce() {
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(b.URL.String() + "/health")
	alive := err == nil && resp.StatusCode == http.StatusOK
	if alive != b.IsAlive() {
		b.SetAlive(alive)
		status := "UP"
		if !alive {
			status = "DOWN"
		}
		log.Printf("backend %s is now %s", b.URL, status)
	}
}
