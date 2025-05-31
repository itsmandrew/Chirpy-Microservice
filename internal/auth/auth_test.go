package auth

import (
	"strings"
	"testing"
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
