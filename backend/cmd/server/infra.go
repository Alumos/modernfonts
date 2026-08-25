package main

import (
	"database/sql"
	"os"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func migrate(db *gorm.DB) error {
	return db.AutoMigrate(
		&DataMigration{},
		&Admin{},
		&SiteSetting{},
		&DocumentSource{},
		&FontItem{},
		&ParseRun{},
	)
}

func openDB(path string) (*gorm.DB, *sql.DB, error) {
	db, err := gorm.Open(sqlite.Open(path), &gorm.Config{})
	if err != nil {
		return nil, nil, err
	}
	sqlDB, err := db.DB()
	if err != nil {
		return nil, nil, err
	}
	sqlDB.SetMaxOpenConns(4)
	sqlDB.SetMaxIdleConns(4)
	if _, err := sqlDB.Exec("PRAGMA journal_mode=WAL"); err != nil {
		return nil, nil, err
	}
	if _, err := sqlDB.Exec("PRAGMA synchronous=NORMAL"); err != nil {
		return nil, nil, err
	}
	if _, err := sqlDB.Exec("PRAGMA busy_timeout=5000"); err != nil {
		return nil, nil, err
	}
	if _, err := sqlDB.Exec("PRAGMA foreign_keys=ON"); err != nil {
		return nil, nil, err
	}
	return db, sqlDB, nil
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
