package main

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestHandleUpdateSiteSavesAndClearsDownloadSettings(t *testing.T) {
	gin.SetMode(gin.TestMode)

	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, sqlDB, err := openDB(dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	if err := migrate(db); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	site := SiteSetting{Name: "字体站"}
	if err := db.Create(&site).Error; err != nil {
		t.Fatalf("create site: %v", err)
	}

	rt := &Runtime{cfg: &Config{}, db: db}
	updateSiteSettings(t, rt, "每日更新精选字体", "font123")
	var got SiteSetting
	if err := db.First(&got, site.ID).Error; err != nil {
		t.Fatalf("load site: %v", err)
	}
	if got.Subtitle != "每日更新精选字体" {
		t.Fatalf("subtitle = %q", got.Subtitle)
	}
	if got.LanzouPassword != "font123" {
		t.Fatalf("lanzou password = %q", got.LanzouPassword)
	}

	updateSiteSettings(t, rt, "", "")
	if err := db.First(&got, site.ID).Error; err != nil {
		t.Fatalf("reload site: %v", err)
	}
	if got.Subtitle != "" {
		t.Fatalf("subtitle after clear = %q", got.Subtitle)
	}
	if got.LanzouPassword != "" {
		t.Fatalf("lanzou password after clear = %q", got.LanzouPassword)
	}
}

func TestHandleStatusDoesNotExposeLanzouPassword(t *testing.T) {
	gin.SetMode(gin.TestMode)

	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, sqlDB, err := openDB(dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	if err := migrate(db); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if err := db.Create(&SiteSetting{Name: "字体站", LanzouPassword: "secret"}).Error; err != nil {
		t.Fatalf("create site: %v", err)
	}

	rt := &Runtime{cfg: &Config{}, db: db}
	req := httptest.NewRequest(http.MethodGet, "/api/system/status", nil)
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = req
	rt.handleStatus(ctx)

	var got map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	site, ok := got["site"].(map[string]any)
	if !ok {
		t.Fatalf("site response = %#v", got["site"])
	}
	if _, exists := site["lanzou_password"]; exists {
		t.Fatal("public status exposed lanzou_password")
	}
}

func updateSiteSettings(t *testing.T, rt *Runtime, subtitle, password string) {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	if err := writer.WriteField("site_name", "字体站"); err != nil {
		t.Fatalf("write site name: %v", err)
	}
	if err := writer.WriteField("site_subtitle", subtitle); err != nil {
		t.Fatalf("write subtitle: %v", err)
	}
	if err := writer.WriteField("lanzou_password", password); err != nil {
		t.Fatalf("write lanzou password: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close form: %v", err)
	}
	req := httptest.NewRequest(http.MethodPut, "/api/admin/settings/site", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = req
	rt.handleUpdateSite(ctx)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
}
