package main

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func (rt *Runtime) registerDomainRoutes(member *gin.RouterGroup, admin *gin.RouterGroup) {
	admin.GET("/sources", rt.handleListSources)
	admin.POST("/sources", rt.handleCreateSource)
	admin.PATCH("/sources/:id", rt.handleUpdateSource)
	admin.DELETE("/sources/:id", rt.handleDeleteSource)
	admin.POST("/sources/:id/parse", rt.handleParseSource)
	admin.GET("/fonts", rt.handleListFonts)
	admin.DELETE("/fonts", rt.handleClearFonts)
	admin.GET("/parse-runs", rt.handleListParseRuns)
	member.POST("/fonts/:id/downloads", rt.handleResolveFontDownloads)
}

func (rt *Runtime) handleListSources(c *gin.Context) {
	_, db, _ := rt.deps()
	sources := make([]DocumentSource, 0)
	if err := db.Order("created_at desc, id desc").Find(&sources).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"sources": sources})
}

func (rt *Runtime) handleCreateSource(c *gin.Context) {
	_, db, _ := rt.deps()
	var req struct {
		URL                    string `json:"url"`
		RefreshIntervalMinutes int    `json:"refresh_interval_minutes"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	req.URL = strings.TrimSpace(req.URL)
	if _, err := extractDocID(req.URL); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.RefreshIntervalMinutes < 1 {
		req.RefreshIntervalMinutes = 60
	}
	next := time.Now().Add(time.Duration(req.RefreshIntervalMinutes) * time.Minute)
	source := DocumentSource{URL: req.URL, RefreshIntervalMinutes: req.RefreshIntervalMinutes, Enabled: true, NextRunAt: &next}
	if err := db.Create(&source).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"source": source})
}

func (rt *Runtime) handleUpdateSource(c *gin.Context) {
	_, db, _ := rt.deps()
	id, _ := strconv.Atoi(c.Param("id"))
	var req struct {
		RefreshIntervalMinutes int   `json:"refresh_interval_minutes"`
		Enabled                *bool `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	var source DocumentSource
	if err := db.First(&source, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "source not found"})
		return
	}
	if req.RefreshIntervalMinutes < 1 {
		req.RefreshIntervalMinutes = source.RefreshIntervalMinutes
	}
	source.RefreshIntervalMinutes = req.RefreshIntervalMinutes
	if req.Enabled != nil {
		source.Enabled = *req.Enabled
	}
	next := time.Now().Add(time.Duration(source.RefreshIntervalMinutes) * time.Minute)
	source.NextRunAt = &next
	if err := db.Save(&source).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"source": source})
}

func (rt *Runtime) handleDeleteSource(c *gin.Context) {
	_, db, _ := rt.deps()
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id < 1 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid source id"})
		return
	}
	if err := db.Transaction(func(tx *gorm.DB) error {
		var source DocumentSource
		if err := tx.First(&source, id).Error; err != nil {
			return err
		}
		if err := tx.Where("source_id = ?", source.ID).Delete(&FontItem{}).Error; err != nil {
			return err
		}
		if err := tx.Where("source_id = ?", source.ID).Delete(&ParseRun{}).Error; err != nil {
			return err
		}
		return tx.Delete(&source).Error
	}); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "source not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (rt *Runtime) handleParseSource(c *gin.Context) {
	_, db, _ := rt.deps()
	id, _ := strconv.Atoi(c.Param("id"))
	var source DocumentSource
	if err := db.First(&source, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "source not found"})
		return
	}
	started := time.Now()
	result, parseErr := rt.parseTencentDoc(c.Request.Context(), source.URL)
	saveResult, err := saveParseResult(db, &source, result, started, parseErr)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if parseErr != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": parseErr.Error(), "result": saveResult})
		return
	}
	c.JSON(http.StatusOK, gin.H{"source": source, "result": saveResult})
}

func saveParseResult(db *gorm.DB, source *DocumentSource, result ParseResult, started time.Time, parseErr error) (gin.H, error) {
	now := time.Now()
	inserted := 0
	status := "success"
	errText := ""
	if parseErr != nil {
		status = "failed"
		errText = parseErr.Error()
	}
	err := db.Transaction(func(tx *gorm.DB) error {
		if parseErr == nil {
			if err := removeInvalidTencentFontRows(tx, source.ID, result.Items); err != nil {
				return err
			}

			var existing []FontItem
			if err := tx.Select("font_name, download_url").Where("source_id = ?", source.ID).Find(&existing).Error; err != nil {
				return err
			}
			existingKeys := make(map[string]struct{}, len(existing))
			for _, font := range existing {
				existingKeys[font.FontName+"\x00"+font.DownloadURL] = struct{}{}
			}

			fonts := make([]FontItem, 0, len(result.Items))
			seenKeys := make(map[string]struct{}, len(result.Items))
			for position, item := range result.Items {
				fontName := cleanFontName(item.FontName)
				if fontName == "" {
					continue
				}
				key := fontName + "\x00" + item.DownloadURL
				if _, seen := seenKeys[key]; seen {
					continue
				}
				seenKeys[key] = struct{}{}
				if _, exists := existingKeys[key]; !exists {
					inserted++
				}
				fonts = append(fonts, FontItem{
					SourceID:         source.ID,
					FontName:         fontName,
					DownloadURL:      item.DownloadURL,
					AccessCode:       item.AccessCode,
					DocumentPosition: position,
					FirstSeenAt:      now,
					LastSeenAt:       now,
				})
			}
			if len(fonts) > 0 {
				if err := tx.Clauses(clause.OnConflict{
					Columns:   []clause.Column{{Name: "source_id"}, {Name: "font_name"}, {Name: "download_url"}},
					DoUpdates: clause.AssignmentColumns([]string{"access_code", "document_position", "last_seen_at", "updated_at"}),
				}).CreateInBatches(fonts, 100).Error; err != nil {
					return err
				}
			}
			if result.Title != "" {
				source.Title = result.Title
			}
			source.LastError = ""
			source.LastParsedAt = &now
		} else {
			source.LastError = errText
		}
		next := now.Add(time.Duration(source.RefreshIntervalMinutes) * time.Minute)
		source.NextRunAt = &next
		updates := map[string]any{
			"last_error":  source.LastError,
			"next_run_at": source.NextRunAt,
		}
		if parseErr == nil {
			updates["last_parsed_at"] = source.LastParsedAt
			if result.Title != "" {
				updates["title"] = source.Title
			}
		}
		updated := tx.Model(&DocumentSource{}).Where("id = ?", source.ID).Updates(updates)
		if updated.Error != nil {
			return updated.Error
		}
		if updated.RowsAffected == 0 {
			return errors.New("document source no longer exists")
		}
		return tx.Create(&ParseRun{
			SourceID:   source.ID,
			Status:     status,
			TotalFound: len(result.Items),
			Inserted:   inserted,
			Error:      errText,
			StartedAt:  started,
			FinishedAt: now,
		}).Error
	})
	if err != nil {
		return nil, err
	}
	return gin.H{"total": len(result.Items), "inserted": inserted}, nil
}

func removeInvalidTencentFontRows(db *gorm.DB, sourceID uint, items []ParsedFont) error {
	parsedLinks := make(map[string]struct{}, len(items))
	parsedPairs := make(map[string]struct{}, len(items))
	for _, item := range items {
		parsedLinks[item.DownloadURL] = struct{}{}
		parsedPairs[cleanFontName(item.FontName)+"\x00"+item.DownloadURL] = struct{}{}
	}
	var existing []FontItem
	if err := db.Select("id, font_name, download_url").Where("source_id = ?", sourceID).Find(&existing).Error; err != nil {
		return err
	}
	for _, font := range existing {
		if _, current := parsedPairs[cleanFontName(font.FontName)+"\x00"+font.DownloadURL]; current {
			continue
		}
		if _, reparsed := parsedLinks[font.DownloadURL]; !reparsed || !isClearlyInvalidTencentFontName(font.FontName) {
			continue
		}
		if err := db.Delete(&font).Error; err != nil {
			return err
		}
	}
	return nil
}

func (rt *Runtime) handleListFonts(c *gin.Context) {
	_, db, _ := rt.deps()
	query := strings.TrimSpace(c.Query("q"))
	sourceID := c.Query("source_id")
	fontQuery := func() *gorm.DB {
		stmt := db.Model(&FontItem{})
		if sourceID != "" {
			stmt = stmt.Where("source_id = ?", sourceID)
		}
		if query != "" {
			stmt = stmt.Where("font_name LIKE ? OR download_url LIKE ?", "%"+query+"%", "%"+query+"%")
		}
		return stmt
	}
	var total int64
	if err := fontQuery().Count(&total).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	limit := queryLimit(c.Query("limit"), 50, 100)
	offset := queryOffset(c.Query("offset"))
	fonts := make([]FontItem, 0, limit)
	if err := fontQuery().
		Select("id, source_id, font_name, download_url, access_code, document_position, first_seen_at, last_seen_at, created_at, updated_at").
		Order("source_id asc, document_position asc, id asc").
		Limit(limit).
		Offset(offset).
		Find(&fonts).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"fonts": fonts, "total": total, "limit": limit, "offset": offset})
}

func (rt *Runtime) handleClearFonts(c *gin.Context) {
	_, db, _ := rt.deps()
	if err := db.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&FontItem{}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (rt *Runtime) handleListParseRuns(c *gin.Context) {
	_, db, _ := rt.deps()
	stmt := db.Order("started_at desc, id desc").Limit(100)
	if sourceID := c.Query("source_id"); sourceID != "" {
		stmt = stmt.Where("source_id = ?", sourceID)
	}
	runs := make([]ParseRun, 0)
	if err := stmt.Find(&runs).Error; err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"runs": runs})
}

func queryLimit(raw string, fallback, maximum int) int {
	value, err := strconv.Atoi(raw)
	if err != nil || value < 1 {
		return fallback
	}
	if value > maximum {
		return maximum
	}
	return value
}

func queryOffset(raw string) int {
	value, err := strconv.Atoi(raw)
	if err != nil || value < 1 {
		return 0
	}
	return value
}
