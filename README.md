# goproxy

HTTP reverse proxy and load balancer in Go with round-robin balancing, per-IP rate limiting, and health checks.

## Run

```bash
go mod tidy

# Start backends (3 separate terminals)
go run backends/server.go -port 9001 -name A
go run backends/server.go -port 9002 -name B
go run backends/server.go -port 9003 -name C

# Start proxy
go run main.go
```

## Try it

```bash
# Round-robin across backends
for i in $(seq 1 6); do curl -s localhost:8080/; done

# Backend status
curl localhost:8080/admin/backends

# Rate limiting (limit: 10 req/s, burst: 20)
for i in $(seq 1 25); do curl -s -o /dev/null -w "%{http_code}\n" localhost:8080/; done
```

## Layout

```
goproxy/
├── main.go                  entrypoint, wires everything
├── proxy/
│   ├── backend.go           Backend struct + health checker
│   ├── balancer.go          Round-robin load balancer
│   ├── ratelimit.go         Per-IP token bucket middleware
│   └── proxy.go             httputil.ReverseProxy wiring
└── backends/
    └── server.go            Test backend server
```

## Next steps

- Connection draining: replace `server.Close()` with `server.Shutdown(ctx)`
- Weighted round-robin: add `Weight int` to Backend
- Sticky sessions: hash session cookie to a fixed backend
- Path-based routing: `map[string]*Balancer` keyed by path prefix
- Circuit breaker: open circuit after N consecutive failures
- Prometheus metrics: request counter + duration histogram on :9090/metrics
