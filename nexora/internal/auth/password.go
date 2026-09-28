// Package auth cuida do hash de senhas e dos tokens de sessão.
package auth

import (
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// Iterations é o fator de trabalho do PBKDF2 usado nos hashes novos.
const Iterations = 100_000

const keyLen = 32

var errMalformed = errors.New("auth: hash de senha malformado")

// HashPassword devolve "pbkdf2-sha256$<iterations>$<salt>$<key>" (base64url, sem padding).
func HashPassword(password string) (string, error) {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("auth: sal: %w", err)
	}
	return hashWithSalt(password, salt, Iterations)
}

func hashWithSalt(password string, salt []byte, iterations int) (string, error) {
	key, err := pbkdf2.Key(sha256.New, password, salt, iterations, keyLen)
	if err != nil {
		return "", fmt.Errorf("auth: derivar a chave: %w", err)
	}
	enc := base64.RawURLEncoding
	return fmt.Sprintf("pbkdf2-sha256$%d$%s$%s", iterations, enc.EncodeToString(salt), enc.EncodeToString(key)), nil
}

// VerifyPassword diz se password confere com o hash codificado.
func VerifyPassword(password, encoded string) (bool, error) {
	parts := strings.Split(encoded, "$")
	if len(parts) != 4 || parts[0] != "pbkdf2-sha256" {
		return false, errMalformed
	}
	iterations, err := strconv.Atoi(parts[1])
	if err != nil || iterations <= 0 {
		return false, errMalformed
	}
	enc := base64.RawURLEncoding
	salt, err := enc.DecodeString(parts[2])
	if err != nil {
		return false, errMalformed
	}
	want, err := enc.DecodeString(parts[3])
	if err != nil {
		return false, errMalformed
	}
	got, err := pbkdf2.Key(sha256.New, password, salt, iterations, len(want))
	if err != nil {
		return false, fmt.Errorf("auth: derivar a chave: %w", err)
	}
	return subtle.ConstantTimeCompare(got, want) == 1, nil
}

// NewToken devolve um token de sessão aleatório, seguro para URL.
func NewToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("auth: token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
