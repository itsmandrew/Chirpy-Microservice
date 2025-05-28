package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
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
func (cfg *apiConfig) metricsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(200)
	fmt.Fprintf(w, `
		<html>
	<body>
		<h1>Welcome, Chirpy Admin</h1>
		<p>Chirpy has been visited %d times!</p>
	</body>
	</html>`, cfg.fileserverHits.Load())
}

// Handler for my reset endpoint, resets the state of our apiConfig, 'hits' to 0
func (cfg *apiConfig) resetHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(200)
	cfg.fileserverHits.Store(0)
	fmt.Fprint(w, "Metrics Reset")
}

func simpleCensor(input string, badWords map[string]struct{}) string {
	// Cleaning up the body now...
	words := strings.Fields(input)
	result := ""

	for i := range words {
		_, ok := badWords[strings.ToLower(words[i])]
		currString := words[i]

		if ok {
			currString = "****"
		}

		result += currString + " "
	}

	result = strings.TrimSpace(result)
	return result
}

func validateChirpHandler(w http.ResponseWriter, r *http.Request) {

	type parameters struct {
		Body string `json:"body"`
	}

	type errorResp struct {
		Error string `json:"error"`
	}

	type goodResponse struct {
		CleanBody string `json:"cleaned_body"`
	}

	bannedWords := map[string]struct{}{
		"kerfuffle": {},
		"sharbert":  {},
		"fornax":    {},
	}

	w.Header().Set("Content-Type", "application/json")
	decoder := json.NewDecoder(r.Body)
	params := parameters{}

	defer r.Body.Close()

	// Checking valid parameters ---------
	err := decoder.Decode(&params)

	if err != nil {
		log.Printf("Error decoding")
		w.WriteHeader(500)

		errBody := errorResp{
			Error: "Something went wrong",
		}

		data, _ := json.Marshal(errBody)
		w.Write(data)

		return
	}

	// ------------------------------------

	log.Printf("Words in payload: %d", len(params.Body))
	if len(params.Body) > 140 {
		log.Printf("Chirp is too long")
		w.WriteHeader(400)

		errBody := errorResp{
			Error: "Chirp is too long",
		}

		data, _ := json.Marshal(errBody)
		w.Write(data)
		return
	}

	result := simpleCensor(params.Body, bannedWords)
	returnResp := goodResponse{
		CleanBody: result,
	}

	data, _ := json.Marshal(returnResp)
	w.WriteHeader(200)
	w.Write(data)

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
	mux.HandleFunc("GET /api/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(200)
		w.Write([]byte("OK"))
	})

	mux.HandleFunc("POST /api/validate_chirp", validateChirpHandler)

	// Check increments endpoint
	mux.HandleFunc(
		"GET /admin/metrics",
		apiCfg.metricsHandler,
	)

	// Reset metrics
	mux.HandleFunc(
		"POST /admin/reset",
		apiCfg.resetHandler,
	)

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
