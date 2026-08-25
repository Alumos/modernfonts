package main

import (
	"testing"
	"time"
)

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
