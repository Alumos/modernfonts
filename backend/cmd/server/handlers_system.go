package main

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

func (rt *Runtime) handleStatus(c *gin.Context) {
	_, db, ok := rt.deps()
	resp := gin.H{"installed": ok}
	if ok {
		var site SiteSetting
		if err := db.First(&site).Error; err == nil {
			resp["site"] = site
		}
	}
	c.JSON(http.StatusOK, resp)
}

func (rt *Runtime) handleHealth(c *gin.Context) {
	if !rt.installed() {
		c.Status(http.StatusServiceUnavailable)
		return
	}
	c.Status(http.StatusNoContent)
}

func (rt *Runtime) handleListPublicFonts(c *gin.Context) {
	_, db, ok := rt.deps()
	if !ok {
		c.JSON(http.StatusOK, gin.H{"fonts": []PublicFontItem{}, "total": 0, "limit": 0, "offset": 0})
		return
	}
	stmt := db.Model(&FontItem{})
	if query := strings.TrimSpace(c.Query("q")); query != "" {
		like := "%" + query + "%"
		stmt = stmt.Where("font_name LIKE ?", like)
	}
	limit := 48
	if value := c.Query("limit"); value != "" {
		if parsed, err := strconv.Atoi(value); err == nil && parsed > 0 {
			if parsed > 1000 {
				parsed = 1000
			}
			limit = parsed
		}
	}
	offset := 0
	if value := c.Query("offset"); value != "" {
		if parsed, err := strconv.Atoi(value); err == nil && parsed > 0 {
			offset = parsed
		}
	}
	var total int64
	if err := stmt.Count(&total).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	fonts := make([]PublicFontItem, 0)
	if err := stmt.
		Select("id, font_name, first_seen_at").
		Order("source_id asc, document_position asc, id asc").
		Limit(limit).
		Offset(offset).
		Scan(&fonts).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"fonts": fonts, "total": total, "limit": limit, "offset": offset})
}

func (rt *Runtime) saveLogo(c *gin.Context) (string, error) {
	file, header, err := c.Request.FormFile("logo")
	if err != nil {
		if errors.Is(err, http.ErrMissingFile) {
			return "", nil
		}
		return "", err
	}
	defer file.Close()
	ext := strings.ToLower(filepath.Ext(header.Filename))
	if ext != ".png" && ext != ".jpg" && ext != ".jpeg" && ext != ".webp" && ext != ".svg" {
		return "", fmt.Errorf("logo must be png, jpg, webp, or svg")
	}
	if err := os.MkdirAll(filepath.Join(rt.dataDir, "uploads"), 0755); err != nil {
		return "", err
	}
	name := "logo" + ext
	dstPath := filepath.Join(rt.dataDir, "uploads", name)
	dst, err := os.Create(dstPath)
	if err != nil {
		return "", err
	}
	defer dst.Close()
	if _, err := io.Copy(dst, io.LimitReader(file, 2<<20)); err != nil {
		return "", err
	}
	return "/uploads/" + name, nil
}
