package main

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func (rt *Runtime) handleGetTencentDocs(c *gin.Context) {
	_, db, _ := rt.deps()
	var credential TencentDocsCredential
	if err := db.Order("id").First(&credential).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusOK, gin.H{"configured": false})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"configured":              true,
		"client_id":               credential.ClientID,
		"open_id":                 credential.OpenID,
		"access_token_expires_at": credential.AccessTokenExpiresAt,
	})
}

func (rt *Runtime) handleUpdateTencentDocs(c *gin.Context) {
	cfg, db, _ := rt.deps()
	var req struct {
		ClientID    string `json:"client_id"`
		AccessToken string `json:"access_token"`
		OpenID      string `json:"open_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	req.ClientID = strings.TrimSpace(req.ClientID)
	req.AccessToken = strings.TrimSpace(req.AccessToken)
	req.OpenID = strings.TrimSpace(req.OpenID)
	if req.ClientID == "" || req.OpenID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "应用 ID 和 Open ID 不能为空"})
		return
	}
	if len(req.ClientID) > 128 || len(req.OpenID) > 128 || len(req.AccessToken) > 8192 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "腾讯文档凭据长度不合法"})
		return
	}

	var credential TencentDocsCredential
	err := db.Order("id").First(&credential).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if errors.Is(err, gorm.ErrRecordNotFound) && req.AccessToken == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Access Token 不能为空"})
		return
	}
	credential.ClientID = req.ClientID
	credential.OpenID = req.OpenID
	if req.AccessToken != "" {
		encrypted, encryptErr := encryptCredential(cfg.JWTSecret, req.AccessToken)
		if encryptErr != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "保存 Access Token 失败"})
			return
		}
		credential.AccessTokenEncrypted = encrypted
		credential.AccessTokenExpiresAt = accessTokenExpiry(req.AccessToken)
	}
	if err := db.Save(&credential).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"ok":                      true,
		"configured":              true,
		"access_token_expires_at": credential.AccessTokenExpiresAt,
	})
}
