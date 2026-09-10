package main

import (
	"encoding/base64"
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestTencentDocsCredentialEncryptionRoundTrip(t *testing.T) {
	secret := strings.Repeat("s", minJWTSecretBytes)
	encrypted, err := encryptCredential(secret, "access-token")
	if err != nil {
		t.Fatalf("encrypt credential: %v", err)
	}
	if encrypted == "access-token" || strings.Contains(encrypted, "access-token") {
		t.Fatal("encrypted credential contains plaintext")
	}
	decrypted, err := decryptCredential(secret, encrypted)
	if err != nil {
		t.Fatalf("decrypt credential: %v", err)
	}
	if decrypted != "access-token" {
		t.Fatalf("decrypted credential = %q", decrypted)
	}
	if _, err := decryptCredential(strings.Repeat("x", minJWTSecretBytes), encrypted); err == nil {
		t.Fatal("decrypt with wrong secret succeeded")
	}
}

func TestAccessTokenExpiryReadsJWTClaim(t *testing.T) {
	want := time.Date(2026, time.October, 10, 6, 16, 41, 0, time.UTC)
	payload := base64.RawURLEncoding.EncodeToString([]byte(fmt.Sprintf(`{"exp":%d}`, want.Unix())))
	got := accessTokenExpiry("header." + payload + ".signature")
	if got == nil || !got.Equal(want) {
		t.Fatalf("expiry = %v, want %v", got, want)
	}
	if got := accessTokenExpiry("opaque-token"); got != nil {
		t.Fatalf("opaque token expiry = %v, want nil", got)
	}
}
