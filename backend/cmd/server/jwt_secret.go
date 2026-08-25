package main

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const jwtSecretFilename = "jwt-secret"
const minJWTSecretBytes = 32

func loadJWTSecret(dataDir string) (string, error) {
	if secret := strings.TrimSpace(os.Getenv("JWT_SECRET")); secret != "" {
		return validateJWTSecret(secret)
	}

	path := filepath.Join(dataDir, jwtSecretFilename)
	secret, err := readJWTSecret(path)
	if err == nil {
		return secret, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return "", err
	}

	random := make([]byte, 32)
	if _, err := rand.Read(random); err != nil {
		return "", fmt.Errorf("generate JWT secret: %w", err)
	}
	secret = base64.RawURLEncoding.EncodeToString(random)
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if errors.Is(err, os.ErrExist) {
		return readJWTSecret(path)
	}
	if err != nil {
		return "", fmt.Errorf("create JWT secret: %w", err)
	}
	if _, err := file.WriteString(secret + "\n"); err != nil {
		_ = file.Close()
		_ = os.Remove(path)
		return "", fmt.Errorf("write JWT secret: %w", err)
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(path)
		return "", fmt.Errorf("close JWT secret: %w", err)
	}
	return secret, nil
}

func readJWTSecret(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	secret := strings.TrimSpace(string(data))
	if secret == "" {
		return "", errors.New("JWT secret file is empty")
	}
	secret, err = validateJWTSecret(secret)
	if err != nil {
		return "", err
	}
	if err := os.Chmod(path, 0600); err != nil {
		return "", fmt.Errorf("protect JWT secret: %w", err)
	}
	return secret, nil
}

func validateJWTSecret(secret string) (string, error) {
	if len(secret) < minJWTSecretBytes {
		return "", fmt.Errorf("JWT secret must contain at least %d bytes", minJWTSecretBytes)
	}
	return secret, nil
}
