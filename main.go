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
	"github.com/itsmandrew/server-go/internal/auth"
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
	jwtSecret       string
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
		Email    string `json:"email"`
		Password string `json:"password"`
	}

	decoder := json.NewDecoder(r.Body)
	params := parameters{}

	defer r.Body.Close()

	err := decoder.Decode(&params)

	// Decoding error print out
	if err != nil {
		log.Printf("Error decoding")
		respondWithError(w, http.StatusInternalServerError, "Something went wrong")
		return
	}

	encryptedPass, err := auth.HashedPassword(params.Password)

	passByParam := database.CreateUserParams{
		Email:          params.Email,
		HashedPassword: encryptedPass,
	}

	// Decoding error print out
	if err != nil {
		log.Printf("Error with encrypting the password")
		respondWithError(w, http.StatusInternalServerError, err.Error())
	}

	user, err := cfg.databaseQueries.CreateUser(r.Context(), passByParam)

	if err != nil {
		log.Printf("CreateUser failed: %v", err)
		respondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}

	log.Printf("Created user: %v\n", user)
	respondWithJson(w, http.StatusCreated, user)
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

	token, err := auth.GetBearerToken(r.Header)

	if err != nil {
		log.Println("No Bearer token")
		respondWithError(w, http.StatusUnauthorized, err.Error())
		return
	}

	userID, err := auth.ValidateJWT(token, cfg.jwtSecret)

	if err != nil {
		log.Println("JWT token is invalid")
		respondWithError(w, http.StatusUnauthorized, err.Error())
		return
	}

	parameters.UserID = userID

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

func (cfg *apiConfig) getChirpsHandler(w http.ResponseWriter, r *http.Request) {

	chirps, err := cfg.databaseQueries.GetChirps(r.Context())

	if err != nil {
		log.Println("Something went wrong with the query")
		respondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}

	log.Printf("Retrieving chirps: %v\n", chirps)
	respondWithJson(w, http.StatusOK, chirps)
}

func (cfg *apiConfig) getIndividualChirpHandler(w http.ResponseWriter, r *http.Request) {

	userID := r.PathValue("chirpID")
	log.Println(userID)

	if userID == "" {
		log.Println("Bad request, no id provided")
		respondWithError(w, http.StatusBadRequest, "No ID provided")
		return
	}

	parsedID, err := uuid.Parse(userID)

	if err != nil {
		log.Println(err.Error())
		respondWithError(w, http.StatusBadRequest, err.Error())
		return
	}

	chirp, err := cfg.databaseQueries.GetIndividualChirp(r.Context(), parsedID)

	if err != nil {
		log.Println("Something went wrong with the query")
		respondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}

	var nullVal database.Chirp

	if chirp == nullVal {
		log.Println("No chirps founds")
		respondWithError(w, http.StatusNotFound, "No chirps found")
		return
	}

	respondWithJson(w, http.StatusOK, chirp)

}

func (cfg *apiConfig) loginUserHandler(w http.ResponseWriter, r *http.Request) {

	type parameters struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}

	type validResponse struct {
		ID           uuid.UUID `json:"id"`
		Email        string    `json:"email"`
		CreatedAt    time.Time `json:"created_at"`
		UpdatedAt    time.Time `json:"updated_at"`
		Token        string    `json:"token"`
		RefreshToken string    `json:"refresh_token"`
	}

	params := parameters{}

	// Decoding logic
	decoder := json.NewDecoder(r.Body)
	defer r.Body.Close()
	err := decoder.Decode(&params)

	if err != nil {
		log.Printf("Error decoding")
		respondWithError(w, http.StatusInternalServerError, "Something went wrong")
		return
	}

	log.Println(params)

	// Get user query (call to database)
	user, err := cfg.databaseQueries.GetUserByEmail(r.Context(), params.Email)

	// Error handling for if the datebase query goes wrong
	if err != nil {
		log.Println("Something went wrong with the query")
		respondWithError(w, http.StatusInternalServerError, "Email does not exist")
		return
	}

	// Checks if our response body password is equal to the encrypted password in our database
	err = auth.CheckPasswordHash(user.HashedPassword, params.Password)

	// Error handling for incorrect password
	if err != nil {
		log.Println(err.Error())
		respondWithError(w, http.StatusUnauthorized, "Email or password is incorrect")
		return
	}

	// Create a JWT token for our user that logins in (access token)
	jwtToken, err := auth.MakeJWT(user.ID, cfg.jwtSecret, time.Duration(3600)*time.Second)

	// Error handling if creation of token fucks up
	if err != nil {
		log.Println("Something went wrong with creating JWT token")
		respondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Create a refresh token (string form)
	refreshToken, _ := auth.MakeRefreshToken()

	refreshTokenParams := database.CreateRefreshTokenParams{
		Token:  refreshToken,
		UserID: user.ID,
	}

	// Insert refresh token into database
	createdRToken, err := cfg.databaseQueries.CreateRefreshToken(r.Context(), refreshTokenParams)

	// Error handling for insert refresh_token into database
	if err != nil {
		log.Println("Something went wrong with inserting refresh token into database")
		respondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}

	log.Printf("Refresh token created for %v\n", user.Email)

	// Everything works
	safeResponse := validResponse{
		ID:           user.ID,
		CreatedAt:    user.CreatedAt,
		UpdatedAt:    user.UpdatedAt,
		Token:        jwtToken,
		RefreshToken: createdRToken.Token,
	}

	respondWithJson(w, http.StatusOK, safeResponse)
}

func (cfg *apiConfig) refreshHandler(w http.ResponseWriter, r *http.Request) {

	type validResponse struct {
		AccessToken string `json:"token"`
	}

	// Check header for the refresh token
	refreshToken, err := auth.GetBearerToken(r.Header)

	// Handling error for missing Authorization token
	if err != nil {
		log.Println("No bearer token")
		respondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Getting the token vals from the database
	dbToken, err := cfg.databaseQueries.GetUserFromRefreshToken(r.Context(), refreshToken)

	// Handling query error (call to database)
	if err != nil {
		log.Println("Error in getting refresh token in database")
		respondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Handling value not found in database (null return)
	var nullToken database.RefreshToken
	if dbToken == nullToken {
		log.Println("Refresh token not found in the database")
		respondWithError(w, http.StatusNotFound, "Refresh token not in database")
		return
	}

	// Creating new access token
	newAccessToken, err := auth.MakeJWT(dbToken.UserID, cfg.jwtSecret, time.Duration(3600)*time.Second)

	// Handling error for creation of access token
	if err != nil {
		log.Println("Error in creating new access/JWT token")
		respondWithJson(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Setting up response
	resp := validResponse{
		AccessToken: newAccessToken,
	}

	// Writing response
	respondWithJson(w, http.StatusOK, resp)

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
	jwtSecret := os.Getenv("JWT_SECRET")

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
		jwtSecret:       jwtSecret,
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

	// Create chirps
	mux.HandleFunc(
		"POST /api/chirps",
		apiCfg.createChirpHandler,
	)

	mux.HandleFunc(
		"GET /api/chirps",
		apiCfg.getChirpsHandler,
	)

	mux.HandleFunc(
		"GET /api/chirps/{chirpID}",
		apiCfg.getIndividualChirpHandler,
	)

	mux.HandleFunc(
		"POST /api/login",
		apiCfg.loginUserHandler,
	)

	mux.HandleFunc(
		"POST /api/refresh",
		apiCfg.refreshHandler,
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
