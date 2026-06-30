package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
)

// GenerateToken creates a new random token with the sdb_ prefix.
// The token is 32 bytes of random data, hex-encoded.
func GenerateToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate token: %w", err)
	}
	return "sdb_" + hex.EncodeToString(b), nil
}

// HashToken computes the SHA-256 hash of the given token, returned as a hex string.
func HashToken(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}
