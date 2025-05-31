package auth

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestHashedPasswordAndCheck(t *testing.T) {
	plain := "myS3cret!"

	// Generate a hash from the plain password
	hash, err := HashedPassword(plain)
	if err != nil {
		t.Fatalf("HashedPassword returned unexpected error: %v", err)
	}

	// Check with the correct password: should return nil
	if err := CheckPasswordHash(hash, plain); err != nil {
		t.Errorf("CheckPasswordHash failed for correct password: %v", err)
	}

	// Check with an incorrect password: should return an error
	if err := CheckPasswordHash(hash, "wrong-password"); err == nil {
		t.Error("expected CheckPasswordHash to return an error for a wrong password, got nil")
	}
}

// Test that HashedPassword returns an error when the password exceeds 72 bytes.
func TestHashedPasswordTooLong(t *testing.T) {
	// Create a password of length 73 bytes
	longPassword := strings.Repeat("x", 73)

	_, err := HashedPassword(longPassword)
	if err == nil {
		t.Error("expected HashedPassword to return an error for a password longer than 72 bytes, got nil")
	}
}

func TestMakeAndValidateJWT(t *testing.T) {

	userID := uuid.New()
	secret := "my-super-secret"
	expiresIn := 5 * time.Minute

	// Make JWT and assert no error and non-empty token strings
	tokenString, err := MakeJWT(userID, secret, expiresIn)

	if err != nil {
		t.Fatalf("MakeJWT returned an unexpected error: %v", err)
	}

	if tokenString == "" {
		t.Fatalf("MakeJWT returned an empty token string")
	}

	parsedID, err := ValidateJWT(tokenString, secret)
	if err != nil {
		t.Fatalf("ValidateJWT returned an unexpected error: %v", err)
	}

	if parsedID != userID {
		t.Errorf("ValidateJWT returned %q; expected %q", parsedID, userID)
	}
}

func TestGetBearerTokenSuccess(t *testing.T) {

	header := http.Header{}
	header.Set("Authorization", "Bearer token123")

	expected := "token123"

	token, err := GetBearerToken(header)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if token != expected {
		t.Errorf("expected to get %v, got %v", expected, token)
	}
}

func TestGetBearerTokenFail(t *testing.T) {
	header := http.Header{}

	token, err := GetBearerToken(header)

	if err == nil {
		t.Fatalf("expected error for missing Authorization header, got token %q", token)
	}
}
