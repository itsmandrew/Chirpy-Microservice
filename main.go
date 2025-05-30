package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/itsmandrew/server-go/internal/database"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

func respondWithJson(w http.ResponseWriter, code int, payload interface{}) error {
	response, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.WriteHeader(code)
	w.Write(response)

	return nil
}

func respondWithError(w http.ResponseWriter, code int, msg string) error {
	return respondWithJson(w, code, map[string]string{"error": msg})
}

// Adjustable struct that allows for state
type apiConfig struct {
	fileserverHits  atomic.Int32
	databaseQueries *database.Queries
	platform        string
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

	type message struct {
		Msg string `json:"msg"`
	}

	if cfg.platform != "dev" {
		w.WriteHeader(http.StatusForbidden)
		return
	}

	// Resetting stuff
	cfg.fileserverHits.Store(0)
	err := cfg.databaseQueries.DeleteUsers(r.Context())

	if err != nil {
		log.Printf("DeleteUsers failed: %v", err)
		respondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}

	msg := message{Msg: "Metrics and users table were reset"}
	respondWithJson(w, http.StatusOK, msg)
	log.Println("Metrics and table reset")
}

// Handler for creating a user
func (cfg *apiConfig) createUserHandler(w http.ResponseWriter, r *http.Request) {

	type parameters struct {
		Email string `json:"email"`
	}

	type User struct {
		ID        uuid.UUID `json:"id"`
		CreatedAt time.Time `json:"created_at"`
		UpdatedAt time.Time `json:"updated_at"`
		Email     string    `json:"email"`
	}

	decoder := json.NewDecoder(r.Body)
	params := parameters{}

	defer r.Body.Close()

	err := decoder.Decode(&params)

	// Decoding error print out
	if err != nil {
		log.Printf("Error decoding")
		respondWithError(w, 500, "Something went wrong")
		return
	}

	user, err := cfg.databaseQueries.CreateUser(r.Context(), params.Email)

	if err != nil {
		log.Printf("CreateUser failed: %v", err)
		respondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}

	log.Printf("Created user: %v\n", user)
	respUser := User{
		ID:        user.ID,
		CreatedAt: user.CreatedAt,
		UpdatedAt: user.UpdatedAt,
		Email:     user.Email,
	}

	respondWithJson(w, http.StatusCreated, respUser)
}

func (cfg *apiConfig) createChirpHandler(w http.ResponseWriter, r *http.Request) {

	var parameters database.CreateChirpParams

	decoder := json.NewDecoder(r.Body)
	defer r.Body.Close()

	err := decoder.Decode(&parameters)

	if err != nil {
		log.Printf("Error decoding")
		respondWithError(w, 500, "Something went wrong")
		return
	}

	ok, cleanBody := validateChirp(parameters.Body)

	if !ok {
		log.Printf("Chirp is too long")
		respondWithError(w, 400, "Chirp is too long")
		return
	}

	parameters.Body = cleanBody

	chirp, err := cfg.databaseQueries.CreateChirp(r.Context(), parameters)

	if err != nil {
		log.Printf("CreateChirp failed: %v", err)
		respondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}

	log.Printf("Created chirp: %v\n", chirp)
	respondWithJson(w, http.StatusCreated, chirp)

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

func validateChirp(body string) (bool, string) {

	bannedWords := map[string]struct{}{
		"kerfuffle": {},
		"sharbert":  {},
		"fornax":    {},
	}

	if len(body) > 140 {
		log.Printf("Chirp is too long")
		return false, ""
	}

	result := simpleCensor(body, bannedWords)
	return true, result
}

func init() {
	// loads .env into the process’s env vars; logs but does not exit if .env is missing
	if err := godotenv.Load(); err != nil {
		log.Println("⚠️  no .env file found, relying on actual environment variables")
	}
}

func main() {

	// Getenv gets the EXPORTED variables, doesn't export
	dbURL := os.Getenv("DB_URL")
	platform := os.Getenv("PLATFORM")

	db, err := sql.Open("postgres", dbURL)

	if err != nil {
		fmt.Println("Cannot connect to db")
		return
	}

	dbQueries := database.New(db)

	// Gives a blank, thread-safe routing table. Ready to attach paths
	// to handler functions, and plug directly into an HTTP server
	// Basically routing, "which code runs for which URL" is handled by ServeMux
	mux := http.NewServeMux()

	apiCfg := apiConfig{
		databaseQueries: dbQueries,
		platform:        platform,
	}

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

	// Create users
	mux.HandleFunc(
		"POST /api/users",
		apiCfg.createUserHandler,
	)

	// cCeate chirps
	mux.HandleFunc(
		"POST /api/chirps",
		apiCfg.createChirpHandler,
	)

	// Server settings for our http server
	server := &http.Server{
		Handler: mux,
		Addr:    ":8080",
	}

	// print on startup:
	log.Printf("Starting server on port %s…", "8080")
	err = server.ListenAndServe()

	if err != nil {
		os.Exit(0)
	}
}
