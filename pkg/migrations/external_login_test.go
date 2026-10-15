package migrations

import (
	"testing"

	"github.com/komari-monitor/komari/database/models"
	appconfig "github.com/komari-monitor/komari/pkg/config"
)

func TestExternalLoginRemovalPreservesPasswordAccess(t *testing.T) {
	db := openTestDB(t, "external_login_removal")
	for _, sql := range []string{
		`CREATE TABLE users (uuid text PRIMARY KEY, username text, passwd text, sso_id text, sso_type text)`,
		`INSERT INTO users VALUES ('owner','admin','preserved-hash','github_secret','github')`,
		`CREATE TABLE sessions (session text PRIMARY KEY, login_method text)`,
		`INSERT INTO sessions VALUES ('password-session','password'),('old-oauth-session','oauth')`,
		`CREATE TABLE oidc_providers (name text PRIMARY KEY, addition text)`,
		`INSERT INTO oidc_providers VALUES ('generic','secret')`,
	} {
		if err := db.Exec(sql).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := db.AutoMigrate(&appconfig.ConfigItem{}, &models.MessageSenderProvider{}); err != nil {
		t.Fatal(err)
	}
	for _, item := range []appconfig.ConfigItem{{Key: "disable_password_login", Value: "true"}, {Key: "o_auth_enabled", Value: "true"}, {Key: "sitename", Value: `"preserved"`}} {
		if err := db.Create(&item).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := db.Create(&models.MessageSenderProvider{Name: "telegram", Addition: `{"bot_token":"keep"}`}).Error; err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if err := Run(Context{DB: db}); err != nil {
			t.Fatal(err)
		}
	}
	if hasTableColumn(db, "users", "sso_id") || hasTableColumn(db, "users", "sso_type") || db.Migrator().HasTable("oidc_providers") {
		t.Fatal("external login data remains")
	}
	var user struct{ Username, Passwd string }
	if err := db.Table("users").First(&user).Error; err != nil || user.Username != "admin" || user.Passwd != "preserved-hash" {
		t.Fatalf("password account changed: %+v %v", user, err)
	}
	var count int64
	db.Table("sessions").Where("session = ?", "password-session").Count(&count)
	if count != 1 {
		t.Fatal("password session removed")
	}
	db.Table("sessions").Where("session = ?", "old-oauth-session").Count(&count)
	if count != 0 {
		t.Fatal("obsolete login session remains")
	}
	db.Model(&appconfig.ConfigItem{}).Where("key IN ?", []string{"disable_password_login", "o_auth_enabled"}).Count(&count)
	if count != 0 {
		t.Fatal("obsolete login config remains")
	}
	var sender models.MessageSenderProvider
	if err := db.First(&sender, "name = ?", "telegram").Error; err != nil || sender.Addition != `{"bot_token":"keep"}` {
		t.Fatal("Telegram config changed")
	}
}
