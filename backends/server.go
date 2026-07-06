package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
)

func main() {
	port := flag.Int("port", 9001, "port to listen on")
	name := flag.String("name", "backend", "backend name for responses")
	flag.Parse()

	mux := http.NewServeMux()

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, "ok")
	})

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		log.Printf("[%s] %s %s", *name, r.Method, r.URL.Path)
		fmt.Fprintf(w, "Hello from %s (port %d)! path=%s\n", *name, *port, r.URL.Path)
	})

	addr := fmt.Sprintf(":%d", *port)
	log.Printf("%s listening on %s", *name, addr)
	log.Fatal(http.ListenAndServe(addr, mux))
}
