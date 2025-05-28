package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"sync/atomic"
)

// Adjustable struct that allows for state
type apiConfig struct {
	fileserverHits atomic.Int32
}

// Wrapper around my other handlers, increments my struct var per request (goroutine) and then handles wrapped handler (using ServeHTTP)
func (cfg *apiConfig) middlewareMetricsInc(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cfg.fileserverHits.Add(1)
		next.ServeHTTP(w, r)
	})
}

// Handler for my metrics endpoint, writes the Content-Type for the heaader and also writes to the body the current "Hits"
func (cfg *apiConfig) handler(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(200)
	fmt.Fprintf(w, "Hits: %v\n", cfg.fileserverHits.Load())
}

// Handler for my reset endpoint, resets the state of our apiConfig, 'hits' to 0
func (cfg *apiConfig) resetHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(200)
	cfg.fileserverHits.Store(0)
	fmt.Fprint(w, "Metrics Reset")
}

func main() {

	// Gives a blank, thread-safe routing table. Ready to attach paths
	// to handler functions, and plug directly into an HTTP server
	// Basically routing, "which code runs for which URL" is handled by ServeMux
	mux := http.NewServeMux()

	apiCfg := apiConfig{}

	// Serving static stuff
	mux.Handle(
		"/app/",
		http.StripPrefix(
			"/app/",
			apiCfg.middlewareMetricsInc(http.FileServer(http.Dir(".")))),
	)

	mux.Handle(
		"/app/assets/",
		http.StripPrefix(
			"/app/assets/",
			apiCfg.middlewareMetricsInc(http.FileServer(http.Dir("./assets"))),
		),
	)

	// Custom response for Health endpoint
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(200)
		w.Write([]byte("OK"))
	})

	// Check increments endpoint
	mux.HandleFunc("GET /metrics", apiCfg.handler)

	// Reset metrics
	mux.HandleFunc("POST /reset", apiCfg.resetHandler)

	// Server settings for our http server
	server := &http.Server{
		Handler: mux,
		Addr:    ":8080",
	}

	// print on startup:
	log.Printf("Starting server on port %s…", "8080")
	err := server.ListenAndServe()

	if err != nil {
		os.Exit(0)
	}
}
