// Package auth generates and hashes the bearer tokens used for
// per-user API authentication. Tokens are never persisted in
// plaintext — only their SHA-256 hash is stored (users.token_hash) —
// so a database leak doesn't directly expose usable credentials.
package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
)

// GenerateToken returns a new random bearer token and its hash. The
// plaintext token is only ever available here, at creation time —
// callers must persist HashToken's result, not the token itself.
func GenerateToken() (token, hash string, err error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", "", fmt.Errorf("generate token: %w", err)
	}
	token = hex.EncodeToString(b)
	return token, HashToken(token), nil
}

// HashToken returns the SHA-256 hash of a bearer token, hex-encoded.
func HashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
