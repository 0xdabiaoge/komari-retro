package migrations

import (
	"fmt"

	appconfig "github.com/komari-monitor/komari/pkg/config"
	"gorm.io/gorm"
)

// Remove secrets and obsolete login-only sessions, preserving passwords and
// password sessions. Run after converting legacy wide configuration tables.
func removeLegacyExternalLogin(db *gorm.DB) error {
	return db.Transaction(func(tx *gorm.DB) error {
		if tx.Migrator().HasTable("oidc_providers") {
			if err := tx.Migrator().DropTable("oidc_providers"); err != nil {
				return err
			}
		}
		for _, column := range []string{"sso_id", "sso_type"} {
			if tx.Migrator().HasTable("users") && hasTableColumn(tx, "users", column) {
				if err := tx.Exec("ALTER TABLE users DROP COLUMN " + column).Error; err != nil {
					return fmt.Errorf("remove %s: %w", column, err)
				}
			}
		}
		if tx.Migrator().HasTable("sessions") && hasTableColumn(tx, "sessions", "login_method") {
			if err := tx.Table("sessions").Where("login_method IN ?", []string{"oauth", "oidc", "sso"}).Delete(nil).Error; err != nil {
				return err
			}
		}
		if tx.Migrator().HasTable("configs") && hasTableColumn(tx, "configs", "key") {
			return tx.Where("key IN ?", []string{"o_auth_enabled", "o_auth_provider", "disable_password_login", "o_auth_client_id", "o_auth_client_secret", "oauth_enabled", "oauth_enable", "oauth_provider"}).Delete(&appconfig.ConfigItem{}).Error
		}
		return nil
	})
}
