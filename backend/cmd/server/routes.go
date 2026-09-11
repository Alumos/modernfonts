package main

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
)

func (rt *Runtime) registerAPI(api *gin.RouterGroup) {
	api.GET("/system/health", rt.handleHealth)
	api.GET("/system/status", rt.handleStatus)
	api.POST("/auth/login", rt.handleLogin)
	api.POST("/auth/logout", rt.handleLogout)
	api.GET("/fonts", rt.handleListPublicFonts)

	session := api.Group("", rt.sessionRequired())
	session.GET("/auth/me", rt.handleMe)

	adminSession := api.Group("/admin", rt.sessionRequired(), rt.adminRequired())
	adminSession.GET("/me", rt.handleMe)
	adminSession.POST("/auth/change-credentials", rt.handleChangeCredentials)

	member := api.Group("", rt.sessionRequired(), rt.credentialsReady())
	member.POST("/archives", rt.handleCreateArchive)
	member.GET("/archives/:id/file", rt.handleArchiveFile)
	member.GET("/archives/:id/font", rt.handleArchiveFont)
	admin := api.Group("/admin", rt.sessionRequired(), rt.adminRequired(), rt.credentialsReady())
	admin.GET("/settings/site", rt.handleGetSite)
	admin.PUT("/settings/site", rt.handleUpdateSite)
	admin.GET("/settings/tencent-docs", rt.handleGetTencentDocs)
	admin.PUT("/settings/tencent-docs", rt.handleUpdateTencentDocs)
	admin.GET("/users", rt.handleListUsers)
	admin.POST("/users", rt.handleCreateUser)
	admin.PATCH("/users/:id/status", rt.handleUpdateUserStatus)
	admin.POST("/users/:id/reset-password", rt.handleResetUserPassword)
	rt.registerDomainRoutes(member, admin)
}

func (rt *Runtime) registerStatic(r *gin.Engine) {
	r.NoRoute(func(c *gin.Context) {
		if strings.HasPrefix(c.Request.URL.Path, "/api/") {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
			return
		}
		path := filepath.Join(rt.publicDir, filepath.Clean(c.Request.URL.Path))
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			c.File(path)
			return
		}
		index := filepath.Join(rt.publicDir, "index.html")
		if _, err := os.Stat(index); err == nil {
			c.File(index)
			return
		}
		c.String(http.StatusOK, "Admin Framework API is running. Frontend assets are not built in this image.")
	})
}
