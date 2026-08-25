package main

import (
	"os"
	"path/filepath"
	"sync"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type Runtime struct {
	mu             sync.RWMutex
	dataDir        string
	publicDir      string
	cfg            *Config
	db             *gorm.DB
	lanzouResolver *nativeLanzouResolver
}

func (rt *Runtime) loadInstalled() error {
	if err := os.MkdirAll(rt.dataDir, 0755); err != nil {
		return err
	}
	jwtSecret, err := loadJWTSecret(rt.dataDir)
	if err != nil {
		return err
	}
	cfg := &Config{JWTSecret: jwtSecret}
	dbPath := env("DB_PATH", filepath.Join(rt.dataDir, "app.db"))
	db, sqlDB, err := openDB(dbPath)
	if err != nil {
		return err
	}
	if err := sqlDB.Ping(); err != nil {
		return err
	}
	if err := migrate(db); err != nil {
		return err
	}
	if err := runDataMigration(db, simplifyFontStorageMigration, applySimplifyFontStorageMigration); err != nil {
		return err
	}
	if err := seedDefaultData(db); err != nil {
		return err
	}
	if err := runDataMigration(db, accountRolesMigration, migrateExistingAccountRoles); err != nil {
		return err
	}
	if err := runDataMigration(db, cleanInvalidFontNamesMigration, cleanInvalidFontNames); err != nil {
		return err
	}
	if err := runDataMigration(db, fontDocumentOrderMigration, backfillFontDocumentPositions); err != nil {
		return err
	}
	rt.mu.Lock()
	rt.cfg, rt.db = cfg, db
	rt.mu.Unlock()
	return nil
}

func seedDefaultData(db *gorm.DB) error {
	var count int64
	if err := db.Model(&Admin{}).Count(&count).Error; err != nil {
		return err
	}
	if count == 0 {
		hash, err := bcrypt.GenerateFromPassword([]byte("123456"), bcrypt.DefaultCost)
		if err != nil {
			return err
		}
		if err := db.Create(&Admin{
			Username:              "admin",
			PasswordHash:          string(hash),
			Role:                  AdminRoleAdmin,
			Status:                AdminStatusActive,
			AuthVersion:           1,
			MustChangeCredentials: true,
		}).Error; err != nil {
			return err
		}
	}
	var siteCount int64
	if err := db.Model(&SiteSetting{}).Count(&siteCount).Error; err != nil {
		return err
	}
	if siteCount == 0 {
		return db.Create(&SiteSetting{Name: "腾讯文档字体解析"}).Error
	}
	return nil
}

func (rt *Runtime) installed() bool {
	rt.mu.RLock()
	defer rt.mu.RUnlock()
	return rt.db != nil
}

func (rt *Runtime) deps() (*Config, *gorm.DB, bool) {
	rt.mu.RLock()
	defer rt.mu.RUnlock()
	return rt.cfg, rt.db, rt.cfg != nil && rt.db != nil
}
