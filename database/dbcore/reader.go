package dbcore

import (
	"strings"
	"sync"

	"github.com/komari-monitor/komari/cmd/flags"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

var readerOnce sync.Once
var readerMu sync.RWMutex
var reader *gorm.DB

// Long history reads use separate read-only WAL connections, so statistics do
// not queue Agent reports and login writes behind a single writer connection.
func GetReadDB() *gorm.DB {
	writer := GetDBInstance()
	readerOnce.Do(func() {
		readerMu.Lock()
		defer readerMu.Unlock()
		if !flags.IsSQLite() || flags.DatabaseFile == ":memory:" || strings.Contains(flags.DatabaseFile, "mode=memory") {
			reader = writer
			return
		}
		dsn := flags.DatabaseFile
		if !strings.HasPrefix(dsn, "file:") {
			dsn = "file:" + dsn
		}
		sep := "?"
		if strings.Contains(dsn, "?") {
			sep = "&"
		}
		var err error
		reader, err = gorm.Open(sqlite.Open(dsn+sep+"mode=ro&_query_only=true&_busy_timeout=1000&_cache_size=-8192"), &gorm.Config{Logger: writer.Logger})
		if err != nil {
			reader = writer
			return
		}
		pool, err := reader.DB()
		if err != nil {
			reader = writer
			return
		}
		pool.SetMaxOpenConns(2)
		pool.SetMaxIdleConns(2)
	})
	readerMu.RLock()
	result := reader
	readerMu.RUnlock()
	return result
}
