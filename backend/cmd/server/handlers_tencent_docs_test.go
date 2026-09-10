package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestTencentDocsSettingsNeverExposeAccessToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rt, db := newAuthTestRuntime(t)
	body := `{"client_id":"client","access_token":"header.` + base64.RawURLEncoding.EncodeToString([]byte(`{"exp":1791613001}`)) + `.signature","open_id":"open"}`
	req := httptest.NewRequest(http.MethodPut, "/api/admin/settings/tencent-docs", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = req
	rt.handleUpdateTencentDocs(ctx)
	if recorder.Code != http.StatusOK {
		t.Fatalf("update status = %d, body = %s", recorder.Code, recorder.Body.String())
	}

	var saved TencentDocsCredential
	if err := db.First(&saved).Error; err != nil {
		t.Fatalf("load credential: %v", err)
	}
	if saved.AccessTokenEncrypted == "" || strings.Contains(saved.AccessTokenEncrypted, "header.") {
		t.Fatal("access token was not encrypted")
	}

	recorder = httptest.NewRecorder()
	ctx, _ = gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/admin/settings/tencent-docs", nil)
	rt.handleGetTencentDocs(ctx)
	if recorder.Code != http.StatusOK {
		t.Fatalf("get status = %d", recorder.Code)
	}
	var response map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if _, exists := response["access_token"]; exists || strings.Contains(recorder.Body.String(), "header.") {
		t.Fatalf("response exposes access token: %s", recorder.Body.String())
	}
}
