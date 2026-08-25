package main

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

const sessionCookieName = "admin_framework_token"

func (rt *Runtime) handleLogin(c *gin.Context) {
	cfg, db, ok := rt.deps()
	if !ok {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "system is not installed"})
		return
	}
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	var admin Admin
	if err := db.Where("username = ? AND status = ?", strings.TrimSpace(req.Username), AdminStatusActive).First(&admin).Error; err != nil ||
		bcrypt.CompareHashAndPassword([]byte(admin.PasswordHash), []byte(req.Password)) != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid username or password"})
		return
	}
	now := time.Now()
	if err := db.Model(&admin).Update("last_login_at", &now).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "cannot update login time"})
		return
	}
	admin.LastLoginAt = &now
	token, err := signToken(cfg.JWTSecret, admin)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "cannot sign token"})
		return
	}
	setSessionCookie(c, token)
	c.JSON(http.StatusOK, accountResponse(admin))
}

func (rt *Runtime) handleChangeCredentials(c *gin.Context) {
	_, db, _ := rt.deps()
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	username, validationError := validateAccountUsername(req.Username)
	if validationError != "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": validationError})
		return
	}
	if validationError := validateAccountPassword(req.Password); validationError != "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": validationError})
		return
	}
	admin, ok := currentAccount(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	if admin.Username == username {
		c.JSON(http.StatusBadRequest, gin.H{"error": "新账号不能和当前账号相同"})
		return
	}
	if bcrypt.CompareHashAndPassword([]byte(admin.PasswordHash), []byte(req.Password)) == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "新密码不能和当前密码相同"})
		return
	}
	var existing int64
	if err := db.Model(&Admin{}).Where("username = ? AND id <> ?", username, admin.ID).Count(&existing).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "cannot check username"})
		return
	}
	if existing > 0 {
		c.JSON(http.StatusConflict, gin.H{"error": "账号已存在"})
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "cannot hash password"})
		return
	}
	updates := map[string]any{
		"username":                username,
		"password_hash":           string(hash),
		"must_change_credentials": false,
		"auth_version":            gorm.Expr("auth_version + 1"),
	}
	if err := db.Model(&Admin{}).Where("id = ?", admin.ID).Updates(updates).Error; err != nil {
		if isUniqueUsernameError(err) {
			c.JSON(http.StatusConflict, gin.H{"error": "账号已存在"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "cannot update credentials"})
		return
	}
	if err := db.First(&admin, admin.ID).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "cannot reload account"})
		return
	}
	token, err := signToken(rt.cfg.JWTSecret, admin)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "cannot sign token"})
		return
	}
	setSessionCookie(c, token)
	c.JSON(http.StatusOK, accountResponse(admin))
}

func (rt *Runtime) handleLogout(c *gin.Context) {
	http.SetCookie(c.Writer, &http.Cookie{Name: sessionCookieName, Value: "", Path: "/", HttpOnly: true, SameSite: http.SameSiteLaxMode, MaxAge: -1})
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (rt *Runtime) sessionRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		cfg, db, ok := rt.deps()
		if !ok {
			c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{"error": "system is not installed"})
			return
		}
		tokenString := ""
		if cookie, err := c.Cookie(sessionCookieName); err == nil {
			tokenString = cookie
		}
		if auth := c.GetHeader("Authorization"); strings.HasPrefix(auth, "Bearer ") {
			tokenString = strings.TrimSpace(strings.TrimPrefix(auth, "Bearer "))
		}
		claims, err := parseAuthToken(cfg.JWTSecret, tokenString)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}
		var admin Admin
		if err := db.First(&admin, claims.AdminID).Error; err != nil || admin.Status != AdminStatusActive || admin.AuthVersion != claims.AuthVersion {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}
		c.Set("admin_id", admin.ID)
		c.Set("admin", &admin)
		c.Set("account", &admin)
		c.Next()
	}
}

func (rt *Runtime) adminRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		admin, ok := currentAccount(c)
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}
		if admin.Role != AdminRoleAdmin {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "无权访问后台"})
			return
		}
		c.Next()
	}
}

func (rt *Runtime) credentialsReady() gin.HandlerFunc {
	return func(c *gin.Context) {
		admin, ok := currentAccount(c)
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}
		if admin.MustChangeCredentials {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "首次登录必须先修改账号和密码", "must_change_credentials": true})
			return
		}
		c.Next()
	}
}

func (rt *Runtime) handleMe(c *gin.Context) {
	admin, ok := currentAccount(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	c.JSON(http.StatusOK, accountResponse(admin))
}

func currentAccount(c *gin.Context) (Admin, bool) {
	value, ok := c.Get("account")
	if !ok {
		return Admin{}, false
	}
	admin, ok := value.(*Admin)
	if !ok || admin == nil {
		return Admin{}, false
	}
	return *admin, true
}

func accountResponse(admin Admin) gin.H {
	response := gin.H{"account": admin, "user": admin}
	if admin.Role == AdminRoleAdmin {
		response["admin"] = admin
	}
	return response
}

func setSessionCookie(c *gin.Context, token string) {
	http.SetCookie(c.Writer, &http.Cookie{Name: sessionCookieName, Value: token, Path: "/", HttpOnly: true, SameSite: http.SameSiteLaxMode, MaxAge: 86400 * 7})
}
