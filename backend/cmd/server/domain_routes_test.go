package main

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func TestHandleDeleteSourceRemovesOnlyRelatedRecords(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rt, db := newAuthTestRuntime(t)
	source := DocumentSource{URL: "https://docs.qq.com/doc/delete", Enabled: true, RefreshIntervalMinutes: 60}
	other := DocumentSource{URL: "https://docs.qq.com/doc/keep", Enabled: true, RefreshIntervalMinutes: 60}
	if err := db.Create(&source).Error; err != nil {
		t.Fatalf("create source: %v", err)
	}
	if err := db.Create(&other).Error; err != nil {
		t.Fatalf("create other source: %v", err)
	}
	for _, item := range []FontItem{
		{SourceID: source.ID, FontName: "待删除字体", DownloadURL: "https://example.com/delete", FirstSeenAt: nowForTest(), LastSeenAt: nowForTest()},
		{SourceID: other.ID, FontName: "保留字体", DownloadURL: "https://example.com/keep", FirstSeenAt: nowForTest(), LastSeenAt: nowForTest()},
	} {
		if err := db.Create(&item).Error; err != nil {
			t.Fatalf("create font: %v", err)
		}
	}
	if err := db.Create(&ParseRun{SourceID: source.ID, Status: "success", StartedAt: nowForTest(), FinishedAt: nowForTest()}).Error; err != nil {
		t.Fatalf("create parse run: %v", err)
	}

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Params = gin.Params{{Key: "id", Value: strconv.Itoa(int(source.ID))}}
	ctx.Request = httptest.NewRequest(http.MethodDelete, "/api/admin/sources/"+strconv.Itoa(int(source.ID)), nil)
	rt.handleDeleteSource(ctx)
	if recorder.Code != http.StatusOK {
		t.Fatalf("delete status = %d, body = %s", recorder.Code, recorder.Body.String())
	}

	for name, model := range map[string]any{
		"source":    &DocumentSource{},
		"font":      &FontItem{},
		"parse run": &ParseRun{},
	} {
		var count int64
		query := db.Model(model)
		if name != "source" {
			query = query.Where("source_id = ?", source.ID)
		} else {
			query = query.Where("id = ?", source.ID)
		}
		if err := query.Count(&count).Error; err != nil {
			t.Fatalf("count %s: %v", name, err)
		}
		if count != 0 {
			t.Fatalf("%s count = %d, want 0", name, count)
		}
	}
	var keptFonts int64
	if err := db.Model(&FontItem{}).Where("source_id = ?", other.ID).Count(&keptFonts).Error; err != nil || keptFonts != 1 {
		t.Fatalf("kept font count = %d, err = %v", keptFonts, err)
	}
}

func TestSaveParseResultDoesNotRecreateDeletedSource(t *testing.T) {
	_, db := newAuthTestRuntime(t)
	source := DocumentSource{URL: "https://docs.qq.com/doc/deleted-during-parse", Enabled: true, RefreshIntervalMinutes: 60}
	if err := db.Create(&source).Error; err != nil {
		t.Fatalf("create source: %v", err)
	}
	if err := db.Delete(&source).Error; err != nil {
		t.Fatalf("delete source: %v", err)
	}
	_, err := saveParseResult(db, &source, ParseResult{Items: []ParsedFont{{
		FontName: "不应写入", DownloadURL: "https://example.com/orphan",
	}}}, time.Now(), nil)
	if err == nil {
		t.Fatal("save result for deleted source succeeded")
	}
	for name, model := range map[string]any{"source": &DocumentSource{}, "font": &FontItem{}, "parse run": &ParseRun{}} {
		var count int64
		if err := db.Model(model).Count(&count).Error; err != nil {
			t.Fatalf("count %s: %v", name, err)
		}
		if count != 0 {
			t.Fatalf("%s count = %d, want 0", name, count)
		}
	}
}

func TestHandleClearParseRunsSupportsCurrentSourceAndAll(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rt, db := newAuthTestRuntime(t)
	for _, run := range []ParseRun{
		{SourceID: 1, Status: "success", StartedAt: nowForTest(), FinishedAt: nowForTest()},
		{SourceID: 1, Status: "failed", StartedAt: nowForTest(), FinishedAt: nowForTest()},
		{SourceID: 2, Status: "success", StartedAt: nowForTest(), FinishedAt: nowForTest()},
	} {
		if err := db.Create(&run).Error; err != nil {
			t.Fatalf("create parse run: %v", err)
		}
	}

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodDelete, "/api/admin/parse-runs?source_id=1", nil)
	rt.handleClearParseRuns(ctx)
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), `"deleted":2`) {
		t.Fatalf("clear source status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	var remaining int64
	if err := db.Model(&ParseRun{}).Count(&remaining).Error; err != nil || remaining != 1 {
		t.Fatalf("remaining runs = %d, err = %v", remaining, err)
	}

	recorder = httptest.NewRecorder()
	ctx, _ = gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodDelete, "/api/admin/parse-runs", nil)
	rt.handleClearParseRuns(ctx)
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), `"deleted":1`) {
		t.Fatalf("clear all status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if err := db.Model(&ParseRun{}).Count(&remaining).Error; err != nil || remaining != 0 {
		t.Fatalf("remaining runs after clear all = %d, err = %v", remaining, err)
	}
}

func TestSaveParseResultRemovesInvalidTencentFontRows(t *testing.T) {
	_, db := newAuthTestRuntime(t)
	source := DocumentSource{URL: "https://docs.qq.com/doc/test", Enabled: true, RefreshIntervalMinutes: 60}
	if err := db.Create(&source).Error; err != nil {
		t.Fatalf("create source: %v", err)
	}
	for _, name := range []string{"2", "ڑ 2"} {
		font := FontItem{
			SourceID:    source.ID,
			FontName:    name,
			DownloadURL: "https://example.lanzou.com/font",
			FirstSeenAt: nowForTest(),
			LastSeenAt:  nowForTest(),
		}
		if err := db.Create(&font).Error; err != nil {
			t.Fatalf("create invalid font %q: %v", name, err)
		}
	}

	result := ParseResult{Items: []ParsedFont{{
		FontName:    "玛莉亚 三字重",
		DownloadURL: "https://example.lanzou.com/font",
		AccessCode:  "abc123",
	}}}
	if _, err := saveParseResult(db, &source, result, time.Now(), nil); err != nil {
		t.Fatalf("save parse result: %v", err)
	}

	var fonts []FontItem
	if err := db.Where("source_id = ?", source.ID).Find(&fonts).Error; err != nil {
		t.Fatalf("load fonts: %v", err)
	}
	if len(fonts) != 1 {
		t.Fatalf("font count = %d, want 1", len(fonts))
	}
	if fonts[0].FontName != "玛莉亚 三字重" || fonts[0].AccessCode != "abc123" {
		t.Fatalf("font = %#v", fonts[0])
	}
}

func TestSaveParseResultTracksDocumentOrder(t *testing.T) {
	_, db := newAuthTestRuntime(t)
	source := DocumentSource{URL: "https://docs.qq.com/doc/order", Enabled: true, RefreshIntervalMinutes: 60}
	if err := db.Create(&source).Error; err != nil {
		t.Fatalf("create source: %v", err)
	}
	items := []ParsedFont{
		{FontName: "字体甲", DownloadURL: "https://example.com/a"},
		{FontName: "字体乙", DownloadURL: "https://example.com/b"},
		{FontName: "字体丙", DownloadURL: "https://example.com/c"},
	}
	if _, err := saveParseResult(db, &source, ParseResult{Items: items}, time.Now(), nil); err != nil {
		t.Fatalf("save first parse: %v", err)
	}
	if _, err := saveParseResult(db, &source, ParseResult{Items: []ParsedFont{items[2], items[0], items[1]}}, time.Now(), nil); err != nil {
		t.Fatalf("save reordered parse: %v", err)
	}

	var fonts []FontItem
	if err := db.Where("source_id = ?", source.ID).Order("document_position asc").Find(&fonts).Error; err != nil {
		t.Fatalf("load fonts: %v", err)
	}
	if len(fonts) != 3 {
		t.Fatalf("font count = %d, want 3", len(fonts))
	}
	for i, want := range []string{"字体丙", "字体甲", "字体乙"} {
		if fonts[i].FontName != want || fonts[i].DocumentPosition != i {
			t.Fatalf("font %d = %#v, want %q at position %d", i, fonts[i], want, i)
		}
	}
}
