package service

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"

	"golang.org/x/crypto/bcrypt"
)

const bcryptCost = bcrypt.DefaultCost

func hashPassword(password string) (string, error) {
	password = strings.TrimSpace(password)

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcryptCost)
	if err != nil {
		return "", err
	}

	return string(hash), nil
}

func comparePassword(hash, password string) (bool, error) {
	password = strings.TrimSpace(password)

	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	if err == nil {
		return true, nil
	}

	if errors.Is(err, bcrypt.ErrMismatchedHashAndPassword) {
		return false, nil
	}

	if errors.Is(err, bcrypt.ErrHashTooShort) {
		return false, nil
	}

	return false, err
}

// passwordVersion fingerprints a stored password hash: every new password
// gets a new bcrypt salt, so the fingerprint changes even when the same
// password is set again.
func passwordVersion(hash string) string {
	sum := sha256.Sum256([]byte(hash))
	return hex.EncodeToString(sum[:8])
}
