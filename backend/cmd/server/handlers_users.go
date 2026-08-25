package main

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

func (rt *Runtime) handleListUsers(c *gin.Context) {
	_, db, _ := rt.deps()
	users := make([]Admin, 0)
	if err := db.Where("role = ?", AdminRoleUser).Order("created_at desc, id desc").Find(&users).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "读取用户列表失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"users": users})
}

func (rt *Runtime) handleCreateUser(c *gin.Context) {
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
	var existing int64
	if err := db.Model(&Admin{}).Where("username = ?", username).Count(&existing).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "检查账号失败"})
		return
	}
	if existing > 0 {
		c.JSON(http.StatusConflict, gin.H{"error": "账号已存在"})
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "生成密码失败"})
		return
	}
	user := Admin{
		Username:              username,
		PasswordHash:          string(hash),
		Role:                  AdminRoleUser,
		Status:                AdminStatusActive,
		AuthVersion:           1,
		MustChangeCredentials: false,
	}
	if err := db.Create(&user).Error; err != nil {
		if isUniqueUsernameError(err) {
			c.JSON(http.StatusConflict, gin.H{"error": "账号已存在"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建用户失败"})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"user": user})
}

func (rt *Runtime) handleUpdateUserStatus(c *gin.Context) {
	_, db, _ := rt.deps()
	userID, ok := parseUserID(c)
	if !ok {
		return
	}
	var req struct {
		Status string `json:"status"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	if req.Status != AdminStatusActive && req.Status != AdminStatusDisabled {
		c.JSON(http.StatusBadRequest, gin.H{"error": "状态只能是 active 或 disabled"})
		return
	}
	user, ok := findManagedUser(c, db, userID)
	if !ok {
		return
	}
	if err := db.Model(&Admin{}).Where("id = ?", user.ID).Updates(map[string]any{
		"status":       req.Status,
		"auth_version": gorm.Expr("auth_version + 1"),
	}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新用户状态失败"})
		return
	}
	if err := db.First(&user, user.ID).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "读取用户失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"user": user})
}

func (rt *Runtime) handleResetUserPassword(c *gin.Context) {
	_, db, _ := rt.deps()
	userID, ok := parseUserID(c)
	if !ok {
		return
	}
	var req struct {
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	if validationError := validateAccountPassword(req.Password); validationError != "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": validationError})
		return
	}
	user, ok := findManagedUser(c, db, userID)
	if !ok {
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "生成密码失败"})
		return
	}
	if err := db.Model(&Admin{}).Where("id = ?", user.ID).Updates(map[string]any{
		"password_hash": string(hash),
		"auth_version":  gorm.Expr("auth_version + 1"),
	}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "重置密码失败"})
		return
	}
	if err := db.First(&user, user.ID).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "读取用户失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"user": user})
}

func parseUserID(c *gin.Context) (uint, bool) {
	value, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || value == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的用户 ID"})
		return 0, false
	}
	return uint(value), true
}

func findManagedUser(c *gin.Context, db *gorm.DB, userID uint) (Admin, bool) {
	var user Admin
	err := db.Where("id = ? AND role = ?", userID, AdminRoleUser).First(&user).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "用户不存在"})
		return Admin{}, false
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "读取用户失败"})
		return Admin{}, false
	}
	return user, true
}
