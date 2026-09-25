// Package auth handles password hashing and session tokens.
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

// Iterations is the PBKDF2 work factor used for new hashes.
const Iterations = 100_000

const keyLen = 32

var errMalformed = errors.New("auth: malformed password hash")

// HashPassword returns "pbkdf2-sha256$<iterations>$<salt>$<key>" (base64url, no padding).
func HashPassword(password string) (string, error) {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("auth: salt: %w", err)
	}
	return hashWithSalt(password, salt, Iterations)
}

func hashWithSalt(password string, salt []byte, iterations int) (string, error) {
	key, err := pbkdf2.Key(sha256.New, password, salt, iterations, keyLen)
	if err != nil {
		return "", fmt.Errorf("auth: derive key: %w", err)
	}
	enc := base64.RawURLEncoding
	return fmt.Sprintf("pbkdf2-sha256$%d$%s$%s", iterations, enc.EncodeToString(salt), enc.EncodeToString(key)), nil
}

// VerifyPassword reports whether password matches the encoded hash.
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
		return false, fmt.Errorf("auth: derive key: %w", err)
	}
	return subtle.ConstantTimeCompare(got, want) == 1, nil
}

// NewToken returns a random, URL-safe session token.
func NewToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("auth: token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
