package proxy

import (
	"errors"
	"sync/atomic"
)

type Balancer struct {
	backends []*Backend
	counter  atomic.Uint64
}

func NewBalancer(backends []*Backend) *Balancer {
	return &Balancer{backends: backends}
}

var ErrNoBackends = errors.New("no healthy backends available")

func (b *Balancer) Next() (*Backend, error) {
	n := uint64(len(b.backends))
	if n == 0 {
		return nil, ErrNoBackends
	}

	start := b.counter.Add(1) - 1
	for i := uint64(0); i < n; i++ {
		backend := b.backends[(start+i)%n]
		if backend.IsAlive() {
			return backend, nil
		}
	}
	return nil, ErrNoBackends
}

func (b *Balancer) Backends() []*Backend {
	return b.backends
}
