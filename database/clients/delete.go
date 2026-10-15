package clients

import (
	"github.com/komari-monitor/komari/database/models"
	"gorm.io/gorm"
)

// Explicit cleanup also works for existing SQLite databases where foreign-key
// enforcement was not enabled. Shared rules and their other nodes are retained.
func deleteClient(db *gorm.DB, uuid string) error {
	return db.Transaction(func(tx *gorm.DB) error {
		for _, table := range []string{"records", "records_long_term", "gpu_records", "ping_records", "offline_notifications", "traffic_report_notifications", "task_results"} {
			if err := tx.Table(table).Where("client = ?", uuid).Delete(nil).Error; err != nil {
				return err
			}
		}
		for _, table := range []string{"ping_tasks", "load_notifications", "tasks"} {
			var rows []struct {
				Clients models.StringArray
			}
			if err := tx.Table(table).Select("clients").Find(&rows).Error; err != nil {
				return err
			}
			for _, row := range rows {
				remaining := make(models.StringArray, 0, len(row.Clients))
				for _, client := range row.Clients {
					if client != uuid {
						remaining = append(remaining, client)
					}
				}
				if len(remaining) == len(row.Clients) {
					continue
				}
				encoded, err := row.Clients.Value()
				if err != nil {
					return err
				}
				if err := tx.Table(table).Where("clients = ? OR clients = ?", encoded, string(encoded.([]byte))).Update("clients", remaining).Error; err != nil {
					return err
				}
			}
		}
		return tx.Delete(&models.Client{}, "uuid = ?", uuid).Error
	})
}
