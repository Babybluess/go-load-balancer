package proxy

import (
	"log"
	"net/http"
	"net/url"
	"sync/atomic"
	"time"
)

// Circuit breaker tuning: after circuitBreakerThreshold consecutive request
// failures, a backend is skipped for circuitBreakerCooldown before a single
// half-open trial request is let through to test for recovery.
const (
	circuitBreakerThreshold = 5
	circuitBreakerCooldown  = 30 * time.Second
)

type Backend struct {
	URL    *url.URL
	Weight int
	alive  atomic.Bool

	failures      atomic.Int32
	circuitOpen   atomic.Bool
	openedAt      atomic.Int64 // UnixNano; valid while circuitOpen is true
	trialInFlight atomic.Bool
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

func (b *Backend) CircuitOpen() bool { return b.circuitOpen.Load() }

// Allowed reports whether a request may be sent to this backend right now.
// A closed circuit always allows traffic. An open circuit blocks traffic
// until the cooldown elapses, at which point it lets exactly one half-open
// trial request through to probe for recovery.
func (b *Backend) Allowed() bool {
	if !b.circuitOpen.Load() {
		return true
	}
	if time.Since(time.Unix(0, b.openedAt.Load())) < circuitBreakerCooldown {
		return false
	}
	return b.trialInFlight.CompareAndSwap(false, true)
}

// RecordSuccess resets the consecutive-failure count and closes the circuit.
func (b *Backend) RecordSuccess() {
	b.failures.Store(0)
	b.trialInFlight.Store(false)
	if b.circuitOpen.CompareAndSwap(true, false) {
		log.Printf("circuit closed for backend %s", b.URL)
	}
}

// RecordFailure increments the consecutive-failure count, opening the
// circuit once circuitBreakerThreshold is reached. A failed half-open trial
// reopens the circuit and restarts the cooldown.
func (b *Backend) RecordFailure() {
	failures := b.failures.Add(1)

	if b.trialInFlight.CompareAndSwap(true, false) {
		b.openedAt.Store(time.Now().UnixNano())
		log.Printf("circuit re-opened for backend %s: half-open trial failed", b.URL)
		return
	}

	if failures >= circuitBreakerThreshold && b.circuitOpen.CompareAndSwap(false, true) {
		b.openedAt.Store(time.Now().UnixNano())
		log.Printf("circuit opened for backend %s after %d consecutive failures", b.URL, failures)
	}
}

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
