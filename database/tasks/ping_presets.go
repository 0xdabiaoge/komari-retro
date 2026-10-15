package tasks

import (
	"fmt"
	"strings"
	"sync"

	"github.com/komari-monitor/komari/database/dbcore"
	"github.com/komari-monitor/komari/database/models"
	"github.com/komari-monitor/komari/pkg/pingpresets"
	"gorm.io/gorm"
)

var pingPresetImportMu sync.Mutex

type PingPresetImportResult struct {
	Created   int    `json:"created"`
	Updated   int    `json:"updated"`
	Unchanged int    `json:"unchanged"`
	TaskIDs   []uint `json:"task_ids"`
}

func ImportPingPresets(ids, clients []string, defaultOn bool, interval int) (PingPresetImportResult, error) {
	pingPresetImportMu.Lock()
	defer pingPresetImportMu.Unlock()
	result, err := importPingPresets(dbcore.GetDBInstance(), ids, clients, defaultOn, interval)
	if err != nil {
		return result, err
	}
	if err = ReloadPingSchedule(); err != nil {
		return result, fmt.Errorf("预设已保存，重载探测计划失败：%w", err)
	}
	return result, nil
}

// One transaction covers the whole selection. Replays merge clients into an
// existing type/target pair while preserving its name, interval and history.
func importPingPresets(db *gorm.DB, ids, clients []string, defaultOn bool, interval int) (PingPresetImportResult, error) {
	result := PingPresetImportResult{TaskIDs: []uint{}}
	if len(ids) == 0 || len(ids) > 256 {
		return result, fmt.Errorf("请选择 1 至 256 个预设")
	}
	if interval < 10 || interval > 86400 {
		return result, fmt.Errorf("探测间隔必须在 10 至 86400 秒之间")
	}
	if len(clients) > 500 {
		return result, fmt.Errorf("单次最多选择 500 台服务器")
	}
	clientIDs := models.StringArray{}
	seenClients := map[string]bool{}
	for _, id := range clients {
		if strings.TrimSpace(id) == "" {
			return result, fmt.Errorf("服务器标识不能为空")
		}
		if !seenClients[id] {
			clientIDs = append(clientIDs, id)
			seenClients[id] = true
		}
	}
	if len(clientIDs) == 0 && !defaultOn {
		return result, fmt.Errorf("请选择服务器，或开启新服务器默认启用")
	}
	selected := []pingpresets.Preset{}
	seen := map[string]bool{}
	for _, id := range ids {
		p, ok := pingpresets.Find(id)
		if !ok {
			return result, fmt.Errorf("未知探测预设：%s", id)
		}
		if !seen[id] {
			selected = append(selected, p)
			seen[id] = true
		}
	}
	err := db.Transaction(func(tx *gorm.DB) error {
		if len(clientIDs) > 0 {
			var count int64
			if err := tx.Model(&models.Client{}).Where("uuid IN ?", []string(clientIDs)).Count(&count).Error; err != nil {
				return err
			}
			if count != int64(len(clientIDs)) {
				return fmt.Errorf("所选服务器不存在或已删除，请刷新后重试")
			}
		}
		var existing []models.PingTask
		if err := tx.Order("id ASC").Find(&existing).Error; err != nil {
			return err
		}
		key := func(kind, target string) string {
			return strings.ToLower(strings.TrimSpace(kind)) + "|" + strings.ToLower(strings.TrimSpace(target))
		}
		lookup := map[string]*models.PingTask{}
		for i := range existing {
			k := key(existing[i].Type, existing[i].Target)
			if lookup[k] == nil {
				lookup[k] = &existing[i]
			}
		}
		for _, p := range selected {
			if old := lookup[key(p.Type, p.Target)]; old != nil {
				merged := append(models.StringArray{}, old.Clients...)
				members := map[string]bool{}
				for _, id := range merged {
					members[id] = true
				}
				for _, id := range clientIDs {
					if !members[id] {
						merged = append(merged, id)
						members[id] = true
					}
				}
				enabled := old.DefaultOn || defaultOn
				if len(merged) != len(old.Clients) || enabled != old.DefaultOn {
					if err := tx.Model(old).Updates(map[string]any{"clients": merged, "all_clients": enabled}).Error; err != nil {
						return err
					}
					old.Clients = merged
					old.DefaultOn = enabled
					result.Updated++
				} else {
					result.Unchanged++
				}
				result.TaskIDs = append(result.TaskIDs, old.Id)
				continue
			}
			task := models.PingTask{Name: p.Name, Type: p.Type, Target: p.Target, Interval: interval, Clients: clientIDs, DefaultOn: defaultOn}
			if err := tx.Create(&task).Error; err != nil {
				return err
			}
			if err := tx.Model(&task).Update("weight", int(task.Id)).Error; err != nil {
				return err
			}
			lookup[key(p.Type, p.Target)] = &task
			result.Created++
			result.TaskIDs = append(result.TaskIDs, task.Id)
		}
		return nil
	})
	if err != nil {
		return PingPresetImportResult{TaskIDs: []uint{}}, err
	}
	return result, nil
}
