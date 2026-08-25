package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func TestHandleHealthReflectsRuntimeState(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tests := []struct {
		name string
		rt   *Runtime
		want int
	}{
		{name: "not ready", rt: &Runtime{}, want: http.StatusServiceUnavailable},
		{name: "ready", rt: &Runtime{db: &gorm.DB{}}, want: http.StatusNoContent},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(rec)
			ctx.Request = httptest.NewRequest(http.MethodGet, "/api/system/health", nil)

			tt.rt.handleHealth(ctx)

			if got := ctx.Writer.Status(); got != tt.want {
				t.Fatalf("status code = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestHandleListPublicFontsReturnsTotalWithLimitedPage(t *testing.T) {
	gin.SetMode(gin.TestMode)

	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, sqlDB, err := openDB(dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() {
		_ = sqlDB.Close()
	})
	if err := migrate(db); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	source := DocumentSource{URL: "https://docs.example.com/fonts", Title: "字体源", Enabled: true, RefreshIntervalMinutes: 60}
	if err := db.Create(&source).Error; err != nil {
		t.Fatalf("create source: %v", err)
	}
	for i := 0; i < 100; i++ {
		font := FontItem{
			SourceID:         source.ID,
			FontName:         fmt.Sprintf("字体%03d", i),
			DownloadURL:      fmt.Sprintf("https://example.com/%03d", i),
			DocumentPosition: i,
			FirstSeenAt:      nowForTest().AddDate(0, 0, i),
			LastSeenAt:       nowForTest().AddDate(0, 0, i),
		}
		if err := db.Create(&font).Error; err != nil {
			t.Fatalf("create font %d: %v", i, err)
		}
	}

	rt := &Runtime{cfg: &Config{}, db: db}
	req := httptest.NewRequest(http.MethodGet, "/api/fonts?limit=36&offset=36", nil)
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = req

	rt.handleListPublicFonts(ctx)

	if rec.Code != http.StatusOK {
		t.Fatalf("status code = %d, body = %s", rec.Code, rec.Body.String())
	}
	var got struct {
		Fonts  []FontItem `json:"fonts"`
		Total  int        `json:"total"`
		Limit  int        `json:"limit"`
		Offset int        `json:"offset"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got.Total != 100 {
		t.Fatalf("total = %d, want 100", got.Total)
	}
	if len(got.Fonts) != 36 {
		t.Fatalf("fonts length = %d, want 36", len(got.Fonts))
	}
	if got.Limit != 36 || got.Offset != 36 {
		t.Fatalf("page = limit:%d offset:%d, want limit:36 offset:36", got.Limit, got.Offset)
	}
	if got.Fonts[0].FontName != "字体036" || got.Fonts[len(got.Fonts)-1].FontName != "字体071" {
		t.Fatalf("font order = %q ... %q, want 字体036 ... 字体071", got.Fonts[0].FontName, got.Fonts[len(got.Fonts)-1].FontName)
	}
}

func TestHandleListPublicFontsReturnsJSONArrayWhenEmpty(t *testing.T) {
	rt, _ := newAuthTestRuntime(t)
	req := httptest.NewRequest(http.MethodGet, "/api/fonts", nil)
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = req

	rt.handleListPublicFonts(ctx)

	if rec.Code != http.StatusOK {
		t.Fatalf("status code = %d, body = %s", rec.Code, rec.Body.String())
	}
	var response struct {
		Fonts json.RawMessage `json:"fonts"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if string(response.Fonts) != "[]" {
		t.Fatalf("fonts JSON = %s, want []", response.Fonts)
	}
}

func TestHandleListFontsReturnsFilteredPage(t *testing.T) {
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

	source := DocumentSource{URL: "https://docs.example.com/fonts", Title: "字体源", Enabled: true, RefreshIntervalMinutes: 60}
	if err := db.Create(&source).Error; err != nil {
		t.Fatalf("create source: %v", err)
	}
	fonts := []FontItem{
		{SourceID: source.ID, FontName: "测试字体一", DownloadURL: "https://example.com/one", FirstSeenAt: nowForTest(), LastSeenAt: nowForTest()},
		{SourceID: source.ID, FontName: "测试字体二", DownloadURL: "https://example.com/two", FirstSeenAt: nowForTest(), LastSeenAt: nowForTest()},
		{SourceID: source.ID, FontName: "其他字体", DownloadURL: "https://example.com/three", FirstSeenAt: nowForTest(), LastSeenAt: nowForTest()},
	}
	if err := db.Create(&fonts).Error; err != nil {
		t.Fatalf("create fonts: %v", err)
	}

	rt := &Runtime{cfg: &Config{}, db: db}
	req := httptest.NewRequest(http.MethodGet, "/api/admin/fonts?q=测试", nil)
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = req
	rt.handleListFonts(ctx)

	if rec.Code != http.StatusOK {
		t.Fatalf("status code = %d, body = %s", rec.Code, rec.Body.String())
	}
	var got struct {
		Fonts  []FontItem `json:"fonts"`
		Total  int        `json:"total"`
		Limit  int        `json:"limit"`
		Offset int        `json:"offset"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(got.Fonts) != 2 || got.Total != 2 || got.Limit != 50 || got.Offset != 0 {
		t.Fatalf("page = fonts:%d total:%d limit:%d offset:%d, want 2/2/50/0", len(got.Fonts), got.Total, got.Limit, got.Offset)
	}
}
