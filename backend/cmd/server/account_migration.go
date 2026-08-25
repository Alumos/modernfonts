package main

import "gorm.io/gorm"

const accountRolesMigration = "20260714-account-roles-v1"

// Every row that predates roles represented an administrator in the old schema.
func migrateExistingAccountRoles(db *gorm.DB) error {
	return db.Model(&Admin{}).Where("1 = 1").Updates(map[string]any{
		"role":         AdminRoleAdmin,
		"auth_version": gorm.Expr("CASE WHEN auth_version < 1 THEN 1 ELSE auth_version END"),
	}).Error
}
