package dbcore

import (
	"github.com/gofrs/flock"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/komari-monitor/komari/cmd/flags"
	"github.com/komari-monitor/komari/database/models"
	"github.com/komari-monitor/komari/pkg/config"
	"github.com/komari-monitor/komari/pkg/migrations"
	logutil "github.com/komari-monitor/komari/utils/log"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

var (
	instance     *gorm.DB
	once         sync.Once
	instanceLock *flock.Flock
)

// Close releases the database and stable process lock during graceful shutdown.
func Close() error {
	var err error
	if instance != nil {
		if sqlDB, e := instance.DB(); e == nil {
			err = sqlDB.Close()
		}
	}
	if instanceLock != nil {
		if e := instanceLock.Unlock(); err == nil {
			err = e
		}
	}
	if databaseLock != nil {
		if e := databaseLock.Unlock(); err == nil {
			err = e
		}
	}
	return err
}

func GetDBInstance() *gorm.DB {
	once.Do(func() {

		var err error
		if err = os.MkdirAll("./data/.runtime", 0700); err != nil {
			log.Fatal(err)
		}
		// Stable inode retained during restore, shared by both host and Docker volumes.
		instanceLock = flock.New("./data/.runtime/instance.lock")
		locked, lockErr := instanceLock.TryLock()
		if lockErr != nil || !locked {
			log.Fatalf("Data directory is already in use: %v", lockErr)
		}
		if err := lockDatabase(flags.DatabaseFile); err != nil {
			log.Fatal(err)
		}
		if err = recoverInterruptedRestore("./data"); err != nil {
			log.Fatal(err)
		}

		if _, err := os.Stat("./data/backup.zip"); err == nil && !RestoreSupported() {
			log.Fatal("Backup restore requires the standard ./data/komari.db database path")
		}
		if err := restorePendingBackup("./data"); err != nil {
			log.Fatalf("Backup validation/activation failed; original data retained: %v", err)
		}

		logConfig := &gorm.Config{
			Logger: logutil.NewGormLogger(),
		}

		// 根据数据库类型选择不同的连接方式
		switch flags.ApplyDatabaseTypeNormalization() {
		case flags.DatabaseTypeSQLite:
			// SQLite 连接
			separator := "?"
			if strings.Contains(flags.DatabaseFile, "?") {
				separator = "&"
			}
			instance, err = gorm.Open(sqlite.Open(flags.DatabaseFile+separator+"_busy_timeout=5000&_journal_mode=WAL&_synchronous=NORMAL"), logConfig)
			if err != nil {
				log.Fatalf("Failed to connect to SQLite3 database: %v", err)
			}
			log.Printf("Using SQLite database file: %s", flags.DatabaseFile)
			sqlDB, poolErr := instance.DB()
			if poolErr != nil {
				log.Fatal(poolErr)
			}
			sqlDB.SetMaxOpenConns(1)
			sqlDB.SetMaxIdleConns(1)
			if err := instance.Exec("PRAGMA journal_mode = WAL;").Error; err != nil {
				log.Printf("Failed to enable WAL mode for SQLite: %v", err)
			}
			instance.Exec("PRAGMA synchronous = NORMAL;")
			instance.Exec("PRAGMA cache_size = -65536;")
			instance.Exec("PRAGMA temp_store = MEMORY;")
			instance.Exec("PRAGMA wal_checkpoint(TRUNCATE);")
		default:
			log.Fatalf("Unsupported database type: %s (supported: %s)", flags.DatabaseType, flags.SupportedDatabaseTypes())
		}
		if err := migrations.Run(migrations.Context{DB: instance}); err != nil {
			log.Fatalf("Failed to run startup migrations: %v", err)
		}
		config.SetDb(instance)
		if theme, _ := config.GetAs[string](config.ThemeKey); strings.EqualFold(theme, "next") || strings.EqualFold(theme, "nasdaq") {
			if err := config.Set(config.ThemeKey, "default"); err != nil {
				log.Fatal(err)
			}
		}

		// 自动迁移模型
		err = instance.AutoMigrate(
			&models.User{},
			&models.Client{},
			&models.Record{},
			&models.GPURecord{},
			&models.Log{},
			&models.Clipboard{},
			&models.LoadNotification{},
			&models.OfflineNotification{},
			&models.TrafficReportNotification{},
			&models.PingRecord{},
			&models.PingTask{},
			&models.OidcProvider{},
			&models.MessageSenderProvider{},
			&models.ThemeConfiguration{},
		)
		if err != nil {
			log.Fatalf("Failed to create tables: %v", err)
		}
		err = instance.Table("records_long_term").AutoMigrate(
			&models.Record{},
		)
		if err != nil {
			log.Fatalf("Failed to create records_long_term table: %v", err)
		}
		err = instance.Table("gpu_records_long_term").AutoMigrate(
			&models.GPURecord{},
		)
		if err != nil {
			log.Fatalf("Failed to create gpu_records_long_term table: %v", err)
		}
		err = instance.AutoMigrate(
			&models.Session{},
		)
		if err != nil {
			log.Fatalf("Failed to create Session table: %v", err)
		}
		err = instance.AutoMigrate(
			&models.Task{},
			&models.TaskResult{},
		)
		if err != nil {
			log.Fatalf("Failed to create Task and TaskResult table: %v", err)
		}

		if err := instance.Where("LOWER(short) IN ?", []string{"next", "nasdaq"}).Delete(&models.ThemeConfiguration{}).Error; err != nil {
			log.Fatal(err)
		}
		entries, err := os.ReadDir("./data/theme")
		if err != nil && !os.IsNotExist(err) {
			log.Fatal(err)
		}
		for _, entry := range entries {
			if strings.EqualFold(entry.Name(), "next") || strings.EqualFold(entry.Name(), "nasdaq") {
				if err := os.RemoveAll(filepath.Join("./data/theme", entry.Name())); err != nil {
					log.Fatal(err)
				}
			}
		}
		if err := completeRestore("./data"); err != nil {
			log.Fatal(err)
		}

		// Manually create composite indexes
		if flags.IsSQLite() {
			instance.Exec("CREATE INDEX IF NOT EXISTS idx_record_client_time ON records(client, time)")
			instance.Exec("CREATE INDEX IF NOT EXISTS idx_record_lt_client_time ON records_long_term(client, time)")
			instance.Exec("CREATE INDEX IF NOT EXISTS idx_gpu_record_client_time ON gpu_records(client, time)")
			instance.Exec("CREATE INDEX IF NOT EXISTS idx_gpu_record_lt_client_time ON gpu_records_long_term(client, time)")
			instance.Exec("CREATE INDEX IF NOT EXISTS idx_ping_record_client_time ON ping_records(client, time)")
		}

	})

	return instance
}
