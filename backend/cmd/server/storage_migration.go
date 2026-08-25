package main

import (
	"strings"

	"gorm.io/gorm"
)

const fontDocumentOrderMigration = "20260825-font-document-order-v1"

const simplifyFontStorageMigration = "20260825-simplify-font-storage-v1"

var legacyStorageTables = []string{
	"font_article_matches",
	"article_images",
	"wechat_articles",
	"wechat_sources",
	"image_collection_runs",
	"site_refresh_runs",
	"audit_logs",
}

var legacyFontColumns = []string{
	"article_url",
	"article_title",
	"published_at",
	"article_match_score",
	"article_match_status",
	"image_status",
	"image_count",
	"image_error",
	"images_collected_at",
}

// simplifyFontStorageMigration keeps the font rows and removes only data that
// belongs to the retired article/image pipeline.
func applySimplifyFontStorageMigration(db *gorm.DB) error {
	for _, table := range legacyStorageTables {
		if err := db.Exec("DROP TABLE IF EXISTS " + table).Error; err != nil {
			return err
		}
	}

	var indexes []struct {
		Name string `gorm:"column:name"`
	}
	if err := db.Raw("PRAGMA index_list('font_items')").Scan(&indexes).Error; err != nil {
		return err
	}
	for _, index := range indexes {
		var columns []struct {
			Name string `gorm:"column:name"`
		}
		quoted := `"` + strings.ReplaceAll(index.Name, `"`, `""`) + `"`
		if err := db.Raw("PRAGMA index_info(" + quoted + ")").Scan(&columns).Error; err != nil {
			return err
		}
		for _, column := range columns {
			if column.Name == "image_status" {
				if err := db.Exec("DROP INDEX IF EXISTS " + quoted).Error; err != nil {
					return err
				}
				break
			}
		}
	}

	var columns []struct {
		Name string `gorm:"column:name"`
	}
	if err := db.Raw("PRAGMA table_info('font_items')").Scan(&columns).Error; err != nil {
		return err
	}
	existing := make(map[string]struct{}, len(columns))
	for _, column := range columns {
		existing[column.Name] = struct{}{}
	}
	for _, column := range legacyFontColumns {
		if _, ok := existing[column]; !ok {
			continue
		}
		if err := db.Exec("ALTER TABLE font_items DROP COLUMN " + column).Error; err != nil {
			return err
		}
	}
	return nil
}

func backfillFontDocumentPositions(db *gorm.DB) error {
	var fonts []FontItem
	if err := db.Select("id, source_id").Order("source_id asc, id asc").Find(&fonts).Error; err != nil {
		return err
	}
	positions := make(map[uint]int)
	for _, font := range fonts {
		position := positions[font.SourceID]
		if err := db.Model(&FontItem{}).Where("id = ?", font.ID).UpdateColumn("document_position", position).Error; err != nil {
			return err
		}
		positions[font.SourceID] = position + 1
	}
	return nil
}
