package main

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

func (rt *Runtime) handleGetSite(c *gin.Context) {
	_, db, _ := rt.deps()
	var site SiteSetting
	db.First(&site)
	c.JSON(http.StatusOK, gin.H{
		"id":              site.ID,
		"name":            site.Name,
		"subtitle":        site.Subtitle,
		"logo_path":       site.LogoPath,
		"lanzou_password": site.LanzouPassword,
		"created_at":      site.CreatedAt,
		"updated_at":      site.UpdatedAt,
	})
}

func (rt *Runtime) handleUpdateSite(c *gin.Context) {
	_, db, _ := rt.deps()
	name := strings.TrimSpace(c.PostForm("site_name"))
	subtitle := strings.TrimSpace(c.PostForm("site_subtitle"))
	updates := map[string]any{"subtitle": subtitle}
	if password, exists := c.GetPostForm("lanzou_password"); exists {
		password = strings.TrimSpace(password)
		if len(password) > 80 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "蓝奏云密码不能超过 80 个字符"})
			return
		}
		updates["lanzou_password"] = password
	}
	if name != "" {
		updates["name"] = name
	}
	if logo, err := rt.saveLogo(c); err == nil && logo != "" {
		updates["logo_path"] = logo
	} else if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if len(updates) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "nothing to update"})
		return
	}
	if err := db.Model(&SiteSetting{}).Where("id = (SELECT id FROM site_settings ORDER BY id LIMIT 1)").Updates(updates).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
