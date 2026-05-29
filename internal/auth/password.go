// Package auth provides authentication for remotty connections.
package auth

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"

	"golang.org/x/crypto/bcrypt"
	"golang.org/x/crypto/argon2"
)

// HashPassword creates a bcrypt hash.
func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("bcrypt hash: %w", err)
	}
	return string(bytes), nil
}

// CheckPassword compares a password against a bcrypt hash.
func CheckPassword(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

// HashPasswordArgon2 creates an Argon2id hash.
func HashPasswordArgon2(password string) (string, error) {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("generate salt: %w", err)
	}

	hash := argon2.IDKey([]byte(password), salt, 1, 64*1024, 4, 32)
	return fmt.Sprintf("$argon2id$v=19$m=65536,t=1,p=4$%s$%s",
		hex.EncodeToString(salt), hex.EncodeToString(hash)), nil
}

// GenerateToken creates a random authentication token.
func GenerateToken(length int) (string, error) {
	b := make([]byte, length)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate token: %w", err)
	}
	return hex.EncodeToString(b), nil
}

// ValidateDeviceID checks if a device ID matches the allow list.
func ValidateDeviceID(deviceID string, allowList []string) bool {
	if len(allowList) == 0 {
		return true // no allow list = allow all
	}
	for _, id := range allowList {
		if id == deviceID || id == "*" {
			return true
		}
	}
	return false
}
