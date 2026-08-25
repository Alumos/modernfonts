package main

import (
	"path/filepath"
	"testing"
)

func TestExistingAccountsMigrateToAdminAndFreshSeedStaysAdmin(t *testing.T) {
	t.Run("existing account", func(t *testing.T) {
		db, sqlDB, err := openDB(filepath.Join(t.TempDir(), "test.db"))
		if err != nil {
			t.Fatalf("open db: %v", err)
		}
		t.Cleanup(func() { _ = sqlDB.Close() })
		if err := migrate(db); err != nil {
			t.Fatalf("migrate schema: %v", err)
		}
		legacy := Admin{Username: "legacy-admin", PasswordHash: "hash", Status: AdminStatusActive}
		if err := db.Create(&legacy).Error; err != nil {
			t.Fatalf("create legacy account: %v", err)
		}
		if legacy.Role != AdminRoleUser {
			t.Fatalf("pre-migration role = %q, want %q", legacy.Role, AdminRoleUser)
		}
		if err := runDataMigration(db, accountRolesMigration, migrateExistingAccountRoles); err != nil {
			t.Fatalf("migrate account roles: %v", err)
		}
		if err := db.First(&legacy, legacy.ID).Error; err != nil {
			t.Fatalf("reload legacy account: %v", err)
		}
		if legacy.Role != AdminRoleAdmin || legacy.AuthVersion != 1 {
			t.Fatalf("migrated account = role:%q version:%d", legacy.Role, legacy.AuthVersion)
		}
	})

	t.Run("fresh seed", func(t *testing.T) {
		db, sqlDB, err := openDB(filepath.Join(t.TempDir(), "test.db"))
		if err != nil {
			t.Fatalf("open db: %v", err)
		}
		t.Cleanup(func() { _ = sqlDB.Close() })
		if err := migrate(db); err != nil {
			t.Fatalf("migrate schema: %v", err)
		}
		if err := seedDefaultData(db); err != nil {
			t.Fatalf("seed data: %v", err)
		}
		var account Admin
		if err := db.Where("username = ?", "admin").First(&account).Error; err != nil {
			t.Fatalf("load seeded admin: %v", err)
		}
		if account.Role != AdminRoleAdmin || !account.MustChangeCredentials || account.AuthVersion != 1 {
			t.Fatalf("seeded account = role:%q must-change:%v version:%d", account.Role, account.MustChangeCredentials, account.AuthVersion)
		}
	})
}
