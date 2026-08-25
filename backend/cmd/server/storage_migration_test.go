package main

import (
	"path/filepath"
	"testing"

	"gorm.io/gorm"
)

func TestSimplifyFontStorageMigrationPreservesCoreRows(t *testing.T) {
	db, sqlDB, err := openDB(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	if err := migrate(db); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	source := DocumentSource{URL: "https://docs.qq.com/doc/test", Enabled: true, RefreshIntervalMinutes: 60}
	if err := db.Create(&source).Error; err != nil {
		t.Fatalf("create source: %v", err)
	}
	font := FontItem{
		SourceID:    source.ID,
		FontName:    "保留字体",
		DownloadURL: "https://example.com/font",
		AccessCode:  "test-pass",
		FirstSeenAt: nowForTest(),
		LastSeenAt:  nowForTest(),
	}
	if err := db.Create(&font).Error; err != nil {
		t.Fatalf("create font: %v", err)
	}

	for _, column := range legacyFontColumns {
		if err := db.Exec("ALTER TABLE font_items ADD COLUMN " + column + " TEXT").Error; err != nil {
			t.Fatalf("add legacy column %s: %v", column, err)
		}
	}
	if err := db.Exec("CREATE INDEX legacy_font_image_index ON font_items(image_status)").Error; err != nil {
		t.Fatalf("create legacy index: %v", err)
	}
	for _, table := range legacyStorageTables {
		if err := db.Exec("CREATE TABLE " + table + " (id INTEGER PRIMARY KEY)").Error; err != nil {
			t.Fatalf("create legacy table %s: %v", table, err)
		}
	}

	if err := runDataMigration(db, simplifyFontStorageMigration, applySimplifyFontStorageMigration); err != nil {
		t.Fatalf("simplify storage: %v", err)
	}
	if err := runDataMigration(db, simplifyFontStorageMigration, func(*gorm.DB) error {
		t.Fatal("storage migration ran twice")
		return nil
	}); err != nil {
		t.Fatalf("repeat migration: %v", err)
	}

	var got FontItem
	if err := db.First(&got, font.ID).Error; err != nil {
		t.Fatalf("load preserved font: %v", err)
	}
	if got.FontName != font.FontName || got.DownloadURL != font.DownloadURL || got.AccessCode != font.AccessCode {
		t.Fatalf("preserved font = %#v", got)
	}

	for _, table := range legacyStorageTables {
		var count int64
		if err := db.Raw("SELECT count(*) FROM sqlite_master WHERE type = 'table' AND name = ?", table).Scan(&count).Error; err != nil {
			t.Fatalf("check legacy table %s: %v", table, err)
		}
		if count != 0 {
			t.Fatalf("legacy table %s still exists", table)
		}
	}
	var columns []struct {
		Name string `gorm:"column:name"`
	}
	if err := db.Raw("PRAGMA table_info('font_items')").Scan(&columns).Error; err != nil {
		t.Fatalf("read font columns: %v", err)
	}
	for _, column := range columns {
		for _, legacy := range legacyFontColumns {
			if column.Name == legacy {
				t.Fatalf("legacy font column %s still exists", legacy)
			}
		}
	}
}

func TestBackfillFontDocumentPositionsOrdersRowsPerSource(t *testing.T) {
	db, sqlDB, err := openDB(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	if err := migrate(db); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	sourceA := DocumentSource{URL: "https://docs.example.com/a", Enabled: true, RefreshIntervalMinutes: 60}
	sourceB := DocumentSource{URL: "https://docs.example.com/b", Enabled: true, RefreshIntervalMinutes: 60}
	if err := db.Create(&sourceA).Error; err != nil {
		t.Fatalf("create source A: %v", err)
	}
	if err := db.Create(&sourceB).Error; err != nil {
		t.Fatalf("create source B: %v", err)
	}
	for _, font := range []FontItem{
		{SourceID: sourceA.ID, FontName: "旧字体一", DownloadURL: "https://example.com/a1", DocumentPosition: 99},
		{SourceID: sourceA.ID, FontName: "旧字体二", DownloadURL: "https://example.com/a2", DocumentPosition: 99},
		{SourceID: sourceB.ID, FontName: "另一字体", DownloadURL: "https://example.com/b1", DocumentPosition: 99},
	} {
		if err := db.Create(&font).Error; err != nil {
			t.Fatalf("create font: %v", err)
		}
	}

	if err := runDataMigration(db, fontDocumentOrderMigration, backfillFontDocumentPositions); err != nil {
		t.Fatalf("backfill document positions: %v", err)
	}
	var fonts []FontItem
	if err := db.Order("source_id asc, document_position asc").Find(&fonts).Error; err != nil {
		t.Fatalf("load fonts: %v", err)
	}
	if len(fonts) != 3 {
		t.Fatalf("font count = %d, want 3", len(fonts))
	}
	for i, want := range []struct {
		name     string
		position int
	}{
		{name: "旧字体一", position: 0},
		{name: "旧字体二", position: 1},
		{name: "另一字体", position: 0},
	} {
		if fonts[i].FontName != want.name || fonts[i].DocumentPosition != want.position {
			t.Fatalf("font %d = %#v, want %q at position %d", i, fonts[i], want.name, want.position)
		}
	}
}
