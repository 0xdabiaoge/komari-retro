package clients

import (
	"testing"
	"time"

	"github.com/komari-monitor/komari/database/models"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func deletionTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.Client{}, &models.Record{}, &models.GPURecord{}, &models.PingTask{}, &models.PingRecord{}, &models.LoadNotification{}, &models.OfflineNotification{}, &models.TrafficReportNotification{}, &models.Task{}, &models.TaskResult{}))
	require.NoError(t, db.Table("records_long_term").AutoMigrate(&models.Record{}))
	return db
}

func TestDeleteClientCleansHistoryAndPreservesSharedRules(t *testing.T) {
	db := deletionTestDB(t)
	for _, id := range []string{"removed", "retained"} {
		require.NoError(t, db.Create(&models.Client{UUID: id, Token: id}).Error)
		for _, table := range []string{"records", "records_long_term", "gpu_records", "offline_notifications", "traffic_report_notifications"} {
			require.NoError(t, db.Table(table).Create(map[string]any{"client": id}).Error)
		}
	}
	ids := models.StringArray{"removed", "retained"}
	require.NoError(t, db.Create(&models.PingTask{Name: "shared", Clients: ids, Target: "127.0.0.1"}).Error)
	require.NoError(t, db.Create(&models.LoadNotification{Name: "shared", Clients: ids}).Error)
	require.NoError(t, db.Create(&models.Task{TaskId: "task", Clients: ids, Command: "echo marker"}).Error)
	require.NoError(t, db.Create(&models.PingRecord{Client: "removed", TaskId: 1, Time: models.FromTime(time.Now()), Value: 5}).Error)
	require.NoError(t, db.Create(&models.TaskResult{TaskId: "task", Client: "removed"}).Error)
	// Old SQLite rows may store the same JSON as TEXT rather than BLOB.
	require.NoError(t, db.Exec(`INSERT INTO ping_tasks(name, clients, target) VALUES ('legacy', '["removed","retained"]', '127.0.0.1')`).Error)
	require.NoError(t, deleteClient(db, "removed"))
	for _, table := range []string{"clients", "records", "records_long_term", "gpu_records", "ping_records", "offline_notifications", "traffic_report_notifications", "task_results"} {
		var count int64
		column := "client"
		if table == "clients" {
			column = "uuid"
		}
		require.NoError(t, db.Table(table).Where(column+" = ?", "removed").Count(&count).Error)
		require.Zero(t, count, table)
	}
	for _, table := range []string{"ping_tasks", "load_notifications", "tasks"} {
		var rows []struct{ Clients models.StringArray }
		require.NoError(t, db.Table(table).Find(&rows).Error)
		require.NotEmpty(t, rows)
		for _, row := range rows {
			require.Equal(t, models.StringArray{"retained"}, row.Clients, table)
		}
	}
	var count int64
	require.NoError(t, db.Table("records").Where("client = ?", "retained").Count(&count).Error)
	require.EqualValues(t, 1, count)
}

func TestDeleteClientRollsBackOnCleanupFailure(t *testing.T) {
	db := deletionTestDB(t)
	require.NoError(t, db.Create(&models.Client{UUID: "kept", Token: "kept"}).Error)
	require.NoError(t, db.Create(&models.Record{Client: "kept"}).Error)
	require.NoError(t, db.Exec("DROP TABLE task_results").Error)
	require.Error(t, deleteClient(db, "kept"))
	var count int64
	require.NoError(t, db.Model(&models.Client{}).Where("uuid = ?", "kept").Count(&count).Error)
	require.EqualValues(t, 1, count)
	require.NoError(t, db.Model(&models.Record{}).Where("client = ?", "kept").Count(&count).Error)
	require.EqualValues(t, 1, count)
}
