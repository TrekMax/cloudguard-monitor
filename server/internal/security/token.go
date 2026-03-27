package security

import (
	"crypto/rand"
	"encoding/hex"
)

// GenerateToken creates a cryptographically random token of the given byte length.
func GenerateToken(byteLen int) (string, error) {
	b := make([]byte, byteLen)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
