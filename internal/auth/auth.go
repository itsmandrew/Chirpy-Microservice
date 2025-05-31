package auth

import (
	"log"

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
