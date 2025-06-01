package auth

import (
	"errors"
	"log"
	"net/http"
	"strings"
	"time"

	"crypto/rand"
	"encoding/hex"

	"github.com/golang-jwt/jwt/v4"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

func HashedPassword(password string) (string, error) {

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)

	if err != nil {
		log.Println("Encrypt failed, see error")
		return "", err
	}

	return string(hash), nil
}

func CheckPasswordHash(hash, password string) error {

	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))

	if err != nil {
		log.Println("Invalid password somehow")
		return err
	}

	return nil
}

func MakeJWT(userID uuid.UUID, tokenSecret string, expiresIn time.Duration) (string, error) {

	now := time.Now().UTC()

	claims := jwt.RegisteredClaims{
		Issuer:    "chirpy",
		IssuedAt:  jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(now.Add(expiresIn)),
		Subject:   userID.String(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	ss, err := token.SignedString([]byte(tokenSecret))

	if err != nil {
		log.Println(err.Error())
		return "", err
	}

	return ss, nil
}

func ValidateJWT(tokenString, tokenSecret string) (uuid.UUID, error) {

	token, err := jwt.ParseWithClaims(tokenString,
		&jwt.RegisteredClaims{},
		func(token *jwt.Token) (interface{}, error) {
			return []byte(tokenSecret), nil
		},
	)

	var nullID uuid.UUID

	if err != nil {
		log.Fatal(err)
		return nullID, err
	}

	subject := token.Claims.(*jwt.RegisteredClaims).Subject
	uid, err := uuid.Parse(subject)

	if err != nil {
		return uuid.UUID{}, err
	}
	return uid, nil
}

func GetBearerToken(headers http.Header) (string, error) {

	token := headers.Get("Authorization")

	if token == "" {
		return "", errors.New("no Authorization field found")
	}

	return strings.TrimPrefix(token, "Bearer "), nil
}

func MakeRefreshToken() (string, error) {

	key := make([]byte, 32)
	rand.Read(key)

	encodedStr := hex.EncodeToString(key)
	return encodedStr, nil
}
