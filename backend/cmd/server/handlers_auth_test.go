package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestAccountRoutePermissionsAndDownloads(t *testing.T) {
	rt, db := newAuthTestRuntime(t)
	admin := createAuthTestAccount(t, db, "admin-test", AdminRoleAdmin)
	user := createAuthTestAccount(t, db, "user-test", AdminRoleUser)
	source := DocumentSource{URL: "https://docs.example.com/fonts", Enabled: true, RefreshIntervalMinutes: 60}
	if err := db.Create(&source).Error; err != nil {
		t.Fatalf("create source: %v", err)
	}
	font := FontItem{
		SourceID:    source.ID,
		FontName:    "测试字体",
		DownloadURL: "https://example.com/private-download",
		AccessCode:  "private-code",
		FirstSeenAt: nowForTest(),
		LastSeenAt:  nowForTest(),
	}
	if err := db.Create(&font).Error; err != nil {
		t.Fatalf("create font: %v", err)
	}
	router := authTestRouter(rt)

	guestDownloads := performAuthTestRequest(t, rt, router, nil, http.MethodPost, "/api/fonts/1/downloads", "")
	if guestDownloads.Code != http.StatusUnauthorized {
		t.Fatalf("guest downloads status = %d, body = %s", guestDownloads.Code, guestDownloads.Body.String())
	}
	userDownloads := performAuthTestRequest(t, rt, router, &user, http.MethodPost, "/api/fonts/invalid/downloads", "")
	if userDownloads.Code != http.StatusBadRequest {
		t.Fatalf("authenticated downloads status = %d, body = %s", userDownloads.Code, userDownloads.Body.String())
	}
	userAdmin := performAuthTestRequest(t, rt, router, &user, http.MethodGet, "/api/admin/users", "")
	if userAdmin.Code != http.StatusForbidden {
		t.Fatalf("user admin status = %d, body = %s", userAdmin.Code, userAdmin.Body.String())
	}
	adminUsers := performAuthTestRequest(t, rt, router, &admin, http.MethodGet, "/api/admin/users", "")
	if adminUsers.Code != http.StatusOK {
		t.Fatalf("admin users status = %d, body = %s", adminUsers.Code, adminUsers.Body.String())
	}
}

func TestSessionRejectsMalformedClaimsWithoutPanicking(t *testing.T) {
	rt, _ := newAuthTestRuntime(t)
	router := authTestRouter(rt)
	claims := jwt.MapClaims{
		"admin_id":     "not-a-number",
		"auth_version": 1,
		"exp":          time.Now().Add(time.Hour).Unix(),
	}
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(rt.cfg.JWTSecret))
	if err != nil {
		t.Fatalf("sign malformed token: %v", err)
	}
	httpRequest := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	httpRequest.Header.Set("Authorization", "Bearer "+token)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httpRequest)
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("malformed token status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
}

func TestPublicFontResponseIsRedacted(t *testing.T) {
	rt, db := newAuthTestRuntime(t)
	source := DocumentSource{URL: "https://docs.example.com/public", Enabled: true, RefreshIntervalMinutes: 60}
	if err := db.Create(&source).Error; err != nil {
		t.Fatalf("create source: %v", err)
	}
	font := FontItem{
		SourceID:    source.ID,
		FontName:    "公开字体",
		DownloadURL: "https://example.com/secret-download",
		AccessCode:  "secret-code",
		FirstSeenAt: nowForTest(),
		LastSeenAt:  nowForTest(),
	}
	if err := db.Create(&font).Error; err != nil {
		t.Fatalf("create font: %v", err)
	}
	recorder := performAuthTestRequest(t, rt, authTestRouter(rt), nil, http.MethodGet, "/api/fonts", "")
	if recorder.Code != http.StatusOK {
		t.Fatalf("public fonts status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	body := recorder.Body.String()
	for _, forbidden := range []string{"download_url", "access_code", "secret-download", "secret-code"} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("public response contains %q: %s", forbidden, body)
		}
	}
	if !strings.Contains(body, "公开字体") {
		t.Fatalf("public response omitted font name: %s", body)
	}
}
