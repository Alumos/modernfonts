package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestJWTSecretIsPersistentAndRejectsWeakOverrides(t *testing.T) {
	t.Setenv("JWT_SECRET", "")
	dir := t.TempDir()
	first, err := loadJWTSecret(dir)
	if err != nil {
		t.Fatalf("create secret: %v", err)
	}
	second, err := loadJWTSecret(dir)
	if err != nil {
		t.Fatalf("reload secret: %v", err)
	}
	if first != second || len(first) < minJWTSecretBytes {
		t.Fatalf("persisted secret mismatch or too short")
	}
	info, err := os.Stat(filepath.Join(dir, jwtSecretFilename))
	if err != nil {
		t.Fatalf("stat secret: %v", err)
	}
	if info.Mode().Perm() != 0600 {
		t.Fatalf("secret mode = %o, want 600", info.Mode().Perm())
	}

	t.Setenv("JWT_SECRET", "too-short")
	if _, err := loadJWTSecret(t.TempDir()); err == nil {
		t.Fatal("weak JWT_SECRET was accepted")
	}
}
