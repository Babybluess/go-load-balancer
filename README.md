# goproxy

HTTP reverse proxy and load balancer in Go with weighted round-robin balancing,
sticky sessions, path-based routing, per-backend circuit breakers, per-IP rate
limiting, health checks, and graceful shutdown.

## Run

```bash
go mod tidy

# Start API backends (3 separate terminals)
go run backends/server.go -port 9001 -name A
go run backends/server.go -port 9002 -name B
go run backends/server.go -port 9003 -name C

# Start static backends (2 separate terminals)
go run backends/server.go -port 9101 -name S1
go run backends/server.go -port 9102 -name S2

# Start proxy
go run main.go
```

Backend pools are configurable via env vars: `API_BACKEND_A/B/C` (default
`localhost:9001-9003`) and `STATIC_BACKEND_A/B` (default `localhost:9101-9102`).

## Try it

```bash
# Round-robin within a pool
for i in $(seq 1 6); do curl -s localhost:8080/api/; done

# Path-based routing: /api/* and /static/* hit independent backend pools
curl -s localhost:8080/api/
curl -s localhost:8080/static/

# Weighted round-robin: set Backend.Weight in main.go and a weight-3
# backend gets picked ~3x as often as a weight-1 backend

# Sticky sessions: same session ID always lands on the same backend
curl -s -H "X-Session-ID: user-42" localhost:8080/api/
curl -s -H "X-Session-ID: user-42" localhost:8080/api/
# ...or via cookie
curl -s -b "session_id=user-42" localhost:8080/api/

# Backend status, grouped by path prefix (flags "(circuit open)" when tripped)
curl localhost:8080/admin/backends

# Circuit breaker: stop a backend so requests to it fail; after 5 consecutive
# failures its circuit opens and traffic skips it for 30s, then one trial
# request is let through to probe for recovery

# Rate limiting (limit: 10 req/s, burst: 20)
for i in $(seq 1 25); do curl -s -o /dev/null -w "%{http_code}\n" localhost:8080/api/; done

# Graceful shutdown: in-flight requests finish (up to 30s) before exit
kill -TERM $(pgrep -f "go run main.go")
```

## Layout

```
goproxy/
├── main.go                  entrypoint, wires backend pools + router
├── proxy/
│   ├── backend.go           Backend struct (URL, Weight) + health checker + circuit breaker
│   ├── balancer.go          Weighted round-robin + sticky-session balancer
│   ├── router.go            Path-prefix routing to per-pool balancers
│   ├── ratelimit.go         Per-IP token bucket middleware
│   └── proxy.go             httputil.ReverseProxy wiring
└── backends/
    └── server.go            Test backend server
```

## Features

- **Weighted round-robin** — `Backend.Weight` controls how often a backend is
  picked; a weight-3 backend is expanded 3x in the balancer's rotation.
- **Sticky sessions** — requests carrying an `X-Session-ID` header or
  `session_id` cookie hash to the same backend on every call, falling
  forward to the next alive backend if their pick is down.
- **Path-based routing** — `Router` maps path prefixes (e.g. `/api/`,
  `/static/`) to independent `Balancer` pools, matched longest-prefix-first.
- **Connection draining** — on `SIGTERM`/`SIGINT`, `server.Shutdown(ctx)`
  waits (up to 30s) for in-flight requests to finish instead of dropping
  them, falling back to `server.Close()` only if the deadline is exceeded.
- **Circuit breaker** — each `Backend` tracks consecutive request failures.
  After 5 in a row its circuit opens and the balancer skips it for a 30s
  cooldown, then lets exactly one half-open trial request through; success
  closes the circuit, failure reopens it and restarts the cooldown.

## Next steps

- Prometheus metrics: request counter + duration histogram on :9090/metrics
