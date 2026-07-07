package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"goproxy/proxy"
)

const shutdownTimeout = 30 * time.Second

func main() {
	apiBackends := mustBuildBackends(
		getEnv("API_BACKEND_A", "http://localhost:9001"),
		getEnv("API_BACKEND_B", "http://localhost:9002"),
		getEnv("API_BACKEND_C", "http://localhost:9003"),
	)
	staticBackends := mustBuildBackends(
		getEnv("STATIC_BACKEND_A", "http://localhost:9101"),
		getEnv("STATIC_BACKEND_B", "http://localhost:9102"),
	)

	apiBalancer := proxy.NewBalancer(apiBackends)
	staticBalancer := proxy.NewBalancer(staticBackends)
	proxy.StartHealthChecks(apiBalancer, 10*time.Second)
	proxy.StartHealthChecks(staticBalancer, 10*time.Second)

	router := proxy.NewRouter(map[string]*proxy.Balancer{
		"/api/":    apiBalancer,
		"/static/": staticBalancer,
	})

	p := proxy.NewProxy(router)
	rl := proxy.NewRateLimiter(10, 20)

	mux := http.NewServeMux()
	mux.Handle("/", rl.Middleware(p))
	mux.HandleFunc("/admin/backends", func(w http.ResponseWriter, r *http.Request) {
		for prefix, balancer := range router.Routes() {
			fmt.Fprintf(w, "== %s ==\n", prefix)
			for _, b := range balancer.Backends() {
				status := "UP"
				if !b.IsAlive() {
					status = "DOWN"
				}
				if b.CircuitOpen() {
					status += " (circuit open)"
				}
				fmt.Fprintf(w, "%s → %s\n", b.URL, status)
			}
		}
	})

	server := &http.Server{
		Addr:         ":8080",
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	metricsMux := http.NewServeMux()
	metricsMux.Handle("/metrics", proxy.MetricsHandler())
	metricsServer := &http.Server{
		Addr:    ":9090",
		Handler: metricsMux,
	}

	go func() {
		log.Println("proxy listening on :8080")
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal(err)
		}
	}()

	go func() {
		log.Println("metrics listening on :9090/metrics")
		if err := metricsServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("metrics server error: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGTERM, syscall.SIGINT)
	<-quit

	log.Println("shutting down proxy, draining in-flight requests...")
	ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Printf("graceful shutdown failed: %v, forcing close", err)
		server.Close()
	}
	if err := metricsServer.Shutdown(ctx); err != nil {
		log.Printf("metrics server shutdown failed: %v", err)
	}
}

func mustBuildBackends(rawURLs ...string) []*proxy.Backend {
	backends := make([]*proxy.Backend, 0, len(rawURLs))
	for _, rawURL := range rawURLs {
		b, err := proxy.NewBackend(rawURL)
		if err != nil {
			log.Fatalf("invalid backend URL %q: %v", rawURL, err)
		}
		backends = append(backends, b)
	}
	return backends
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
