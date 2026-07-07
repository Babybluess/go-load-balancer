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
	backendURLs := []string{
		getEnv("BACKEND_A", "http://localhost:9001"),
		getEnv("BACKEND_B", "http://localhost:9002"),
		getEnv("BACKEND_C", "http://localhost:9003"),
	}

	var backends []*proxy.Backend
	for _, rawURL := range backendURLs {
		b, err := proxy.NewBackend(rawURL)
		if err != nil {
			log.Fatalf("invalid backend URL %q: %v", rawURL, err)
		}
		backends = append(backends, b)
	}

	balancer := proxy.NewBalancer(backends)
	proxy.StartHealthChecks(balancer, 10*time.Second)

	p := proxy.NewProxy(balancer)
	rl := proxy.NewRateLimiter(10, 20)

	mux := http.NewServeMux()
	mux.Handle("/", rl.Middleware(p))
	mux.HandleFunc("/admin/backends", func(w http.ResponseWriter, r *http.Request) {
		for _, b := range balancer.Backends() {
			status := "UP"
			if !b.IsAlive() {
				status = "DOWN"
			}
			fmt.Fprintf(w, "%s → %s\n", b.URL, status)
		}
	})

	server := &http.Server{
		Addr:         ":8080",
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	go func() {
		log.Println("proxy listening on :8080")
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal(err)
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
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
