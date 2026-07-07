package proxy

import (
	"errors"
	"sync/atomic"
)

type Balancer struct {
	backends []*Backend
	weighted []*Backend
	counter  atomic.Uint64
}

func NewBalancer(backends []*Backend) *Balancer {
	return &Balancer{
		backends: backends,
		weighted: expandByWeight(backends),
	}
}

// expandByWeight repeats each backend Weight times so that Next() favors
// higher-weight backends proportionally in its round-robin rotation.
func expandByWeight(backends []*Backend) []*Backend {
	var expanded []*Backend
	for _, backend := range backends {
		weight := max(backend.Weight, 1)
		for range weight {
			expanded = append(expanded, backend)
		}
	}
	return expanded
}

var ErrNoBackends = errors.New("no healthy backends available")

func (b *Balancer) Next() (*Backend, error) {
	n := uint64(len(b.weighted))
	if n == 0 {
		return nil, ErrNoBackends
	}

	start := b.counter.Add(1) - 1
	for i := range n {
		backend := b.weighted[(start+i)%n]
		if backend.IsAlive() {
			return backend, nil
		}
	}
	return nil, ErrNoBackends
}

func (b *Balancer) Backends() []*Backend {
	return b.backends
}
