package main

import (
	"path/filepath"
	"testing"

	"gorm.io/gorm"
)

func TestRunDataMigrationRunsOnce(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, sqlDB, err := openDB(dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	if err := migrate(db); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	calls := 0
	apply := func(_ *gorm.DB) error {
		calls++
		return nil
	}
	for i := 0; i < 2; i++ {
		if err := runDataMigration(db, "test-migration", apply); err != nil {
			t.Fatalf("run migration %d: %v", i+1, err)
		}
	}
	if calls != 1 {
		t.Fatalf("migration calls = %d, want 1", calls)
	}
}

func TestCleanInvalidFontNamesUpdatesExistingDirtyRecords(t *testing.T) {
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
	font := FontItem{
		SourceID:    source.ID,
		FontName:    "� 果酱丸 5字重100/90",
		DownloadURL: "https://example.com/font",
		FirstSeenAt: nowForTest(),
		LastSeenAt:  nowForTest(),
	}
	if err := db.Create(&font).Error; err != nil {
		t.Fatalf("create font: %v", err)
	}

	if err := cleanInvalidFontNames(db); err != nil {
		t.Fatalf("clean invalid font names: %v", err)
	}

	var got FontItem
	if err := db.First(&got, font.ID).Error; err != nil {
		t.Fatalf("load font: %v", err)
	}
	if got.FontName != "果酱丸 5字重100/90" {
		t.Fatalf("font name = %q, want %q", got.FontName, "果酱丸 5字重100/90")
	}
}

func TestCleanInvalidFontNamesDeletesEmptyDirtyRecords(t *testing.T) {
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
	font := FontItem{
		SourceID:    source.ID,
		FontName:    "�",
		DownloadURL: "https://example.com/bad-font",
		FirstSeenAt: nowForTest(),
		LastSeenAt:  nowForTest(),
	}
	if err := db.Create(&font).Error; err != nil {
		t.Fatalf("create font: %v", err)
	}

	if err := cleanInvalidFontNames(db); err != nil {
		t.Fatalf("clean invalid font names: %v", err)
	}

	var count int64
	if err := db.Model(&FontItem{}).Where("id = ?", font.ID).Count(&count).Error; err != nil {
		t.Fatalf("count font: %v", err)
	}
	if count != 0 {
		t.Fatalf("font count = %d, want 0", count)
	}
}
