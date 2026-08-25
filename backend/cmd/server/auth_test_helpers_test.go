package main

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

func newAuthTestRuntime(t *testing.T) (*Runtime, *gorm.DB) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db, sqlDB, err := openDB(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	if err := migrate(db); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return &Runtime{cfg: &Config{JWTSecret: strings.Repeat("s", minJWTSecretBytes)}, db: db}, db
}

func nowForTest() time.Time {
	return time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)
}

func createAuthTestAccount(t *testing.T, db *gorm.DB, username, role string) Admin {
	t.Helper()
	hash, err := bcrypt.GenerateFromPassword([]byte("password-123"), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	account := Admin{
		Username:     username,
		PasswordHash: string(hash),
		Role:         role,
		Status:       AdminStatusActive,
		AuthVersion:  1,
	}
	if err := db.Create(&account).Error; err != nil {
		t.Fatalf("create account: %v", err)
	}
	return account
}

func authTestRouter(rt *Runtime) *gin.Engine {
	router := gin.New()
	rt.registerAPI(router.Group("/api"))
	return router
}

func performAuthTestRequest(t *testing.T, rt *Runtime, router http.Handler, account *Admin, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if account != nil {
		token, err := signToken(rt.cfg.JWTSecret, *account)
		if err != nil {
			t.Fatalf("sign token: %v", err)
		}
		req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: token})
	}
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)
	return recorder
}
