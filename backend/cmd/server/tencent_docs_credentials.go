package main

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"gorm.io/gorm"
)

type tencentDocsAuth struct {
	ClientID    string
	AccessToken string
	OpenID      string
	ExpiresAt   *time.Time
}

func credentialKey(secret string) [32]byte {
	return sha256.Sum256([]byte("modernfonts/tencent-docs/v1\x00" + secret))
}

func encryptCredential(secret, plaintext string) (string, error) {
	if secret == "" || plaintext == "" {
		return "", errors.New("credential secret and value are required")
	}
	key := credentialKey(secret)
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	sealed := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.RawURLEncoding.EncodeToString(sealed), nil
}

func decryptCredential(secret, encoded string) (string, error) {
	sealed, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		return "", fmt.Errorf("decode credential: %w", err)
	}
	key := credentialKey(secret)
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	if len(sealed) < gcm.NonceSize() {
		return "", errors.New("encrypted credential is incomplete")
	}
	plaintext, err := gcm.Open(nil, sealed[:gcm.NonceSize()], sealed[gcm.NonceSize():], nil)
	if err != nil {
		return "", fmt.Errorf("decrypt credential: %w", err)
	}
	return string(plaintext), nil
}

func accessTokenExpiry(token string) *time.Time {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil
	}
	var claims struct {
		ExpiresAt float64 `json:"exp"`
	}
	if err := json.Unmarshal(payload, &claims); err != nil || claims.ExpiresAt <= 0 {
		return nil
	}
	expiresAt := time.Unix(int64(claims.ExpiresAt), 0)
	return &expiresAt
}

func loadTencentDocsAuth(db *gorm.DB, cfg *Config) (*tencentDocsAuth, error) {
	var credential TencentDocsCredential
	if err := db.Order("id").First(&credential).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	token, err := decryptCredential(cfg.JWTSecret, credential.AccessTokenEncrypted)
	if err != nil {
		return nil, err
	}
	return &tencentDocsAuth{
		ClientID:    credential.ClientID,
		AccessToken: token,
		OpenID:      credential.OpenID,
		ExpiresAt:   credential.AccessTokenExpiresAt,
	}, nil
}
