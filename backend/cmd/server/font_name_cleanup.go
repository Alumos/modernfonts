package main

import (
	"errors"
	"strings"
	"unicode/utf8"

	"gorm.io/gorm"
)

const cleanInvalidFontNamesMigration = "20260714-clean-invalid-font-names"

func cleanFontName(value string) string {
	var b strings.Builder
	for _, r := range value {
		if r == utf8.RuneError {
			b.WriteRune(' ')
			continue
		}
		b.WriteRune(r)
	}
	return strings.TrimSpace(strings.Join(strings.Fields(b.String()), " "))
}

func cleanInvalidFontNames(db *gorm.DB) error {
	var fonts []FontItem
	if err := db.Where("font_name LIKE ?", "%"+string(utf8.RuneError)+"%").Find(&fonts).Error; err != nil {
		return err
	}
	for _, font := range fonts {
		cleaned := cleanFontName(font.FontName)
		if cleaned == "" {
			if err := db.Delete(&font).Error; err != nil {
				return err
			}
			continue
		}
		if cleaned == font.FontName {
			continue
		}
		var existing FontItem
		err := db.
			Where("source_id = ? AND font_name = ? AND download_url = ? AND id <> ?", font.SourceID, cleaned, font.DownloadURL, font.ID).
			First(&existing).Error
		if err == nil {
			updates := map[string]any{
				"last_seen_at": font.LastSeenAt,
				"access_code":  font.AccessCode,
			}
			if err := db.Model(&existing).Updates(updates).Error; err != nil {
				return err
			}
			if err := db.Delete(&font).Error; err != nil {
				return err
			}
			continue
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if err := db.Model(&font).Update("font_name", cleaned).Error; err != nil {
			return err
		}
	}
	return nil
}

func runDataMigration(db *gorm.DB, name string, apply func(*gorm.DB) error) error {
	return db.Transaction(func(tx *gorm.DB) error {
		var applied int64
		if err := tx.Model(&DataMigration{}).Where("name = ?", name).Count(&applied).Error; err != nil {
			return err
		}
		if applied > 0 {
			return nil
		}
		if err := apply(tx); err != nil {
			return err
		}
		return tx.Create(&DataMigration{Name: name}).Error
	})
}
