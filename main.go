package main

import (
	"log"
	"net/http"
	"os"
)

func main() {

	mux := http.NewServeMux()

	// Serving static stuff
	mux.Handle(
		"/app/",
		http.StripPrefix("/app", http.FileServer(http.Dir("."))),
	)

	mux.Handle(
		"/app/assets/",
		http.StripPrefix(
			"/app/assets/",
			http.FileServer(http.Dir("./assets")),
		),
	)

	// Custom response for Health endpoint
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(200)
		w.Write([]byte("OK"))
	})

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
