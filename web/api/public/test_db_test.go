package public

import (
	"os"
	"testing"

	"github.com/komari-monitor/komari/cmd/flags"
	"github.com/komari-monitor/komari/database/dbcore"
)

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "komari-public-tests-*")
	if err != nil {
		panic(err)
	}
	previous, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	if err := os.Chdir(dir); err != nil {
		panic(err)
	}
	flags.DatabaseType = flags.DatabaseTypeSQLite
	flags.DatabaseFile = "file:web_api_public_test?mode=memory&cache=shared"

	db := dbcore.GetDBInstance()
	if sqlDB, err := db.DB(); err == nil {
		sqlDB.SetMaxOpenConns(1)
	}

	status := m.Run()
	_ = dbcore.Close()
	_ = os.Chdir(previous)
	_ = os.RemoveAll(dir)
	os.Exit(status)
}
