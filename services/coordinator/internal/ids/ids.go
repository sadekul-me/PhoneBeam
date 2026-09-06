package ids

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
)

func RandomID(byteLen int) (string, error) {
	buf := make([]byte, byteLen)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("secure random: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func HashToken(token string) [32]byte {
	return sha256.Sum256([]byte(token))
}

func EqualHash(a, b [32]byte) bool {
	acc := byte(0)
	for i := range a {
		acc |= a[i] ^ b[i]
	}
	return acc == 0
}
