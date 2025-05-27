package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"sync/atomic"
)

type apiConfig struct {
	fileserverHits atomic.Int32
}

func (cfg *apiConfig) middlewareMetricsInc(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cfg.fileserverHits.Add(1)
		next.ServeHTTP(w, r)
	})
}

func (cfg *apiConfig) handler(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(200)
	fmt.Fprintf(w, "Hits: %v\n", cfg.fileserverHits.Load())
}

func (cfg *apiConfig) resetHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(200)
	cfg.fileserverHits.Store(0)
	fmt.Fprint(w, "Metrics Reset")
}

func main() {

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
