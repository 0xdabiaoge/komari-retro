package tasks

import (
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"unicode"
	"unicode/utf8"

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

type PingPresetOverride struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Type   string `json:"type"`
	Target string `json:"target"`
}

func pingPresetTargetKey(kind, target string) string {
	kind, target = strings.ToLower(strings.TrimSpace(kind)), strings.TrimSpace(target)
	if kind == "http" {
		if u, err := url.Parse(target); err == nil {
			u.Scheme, u.Host = strings.ToLower(u.Scheme), strings.ToLower(u.Host)
			target = u.String()
		}
		// HTTP paths and queries are case-sensitive; only the authority is folded.
		return kind + "|" + target
	}
	return kind + "|" + strings.ToLower(target)
}

func validatePingPresetOverride(p PingPresetOverride) error {
	if p.Name == "" || utf8.RuneCountInString(p.Name) > 128 || strings.ContainsFunc(p.Name, unicode.IsControl) {
		return fmt.Errorf("探测名称须为 1 至 128 个字符，且不能包含控制字符")
	}
	if p.Target == "" || len(p.Target) > 2048 || strings.ContainsFunc(p.Target, unicode.IsSpace) || strings.ContainsFunc(p.Target, unicode.IsControl) {
		return fmt.Errorf("探测地址不能为空、不能包含空白，且不能超过 2048 字节")
	}
	hostOK := func(host string) bool { return host != "" && !strings.ContainsAny(host, "/\\?#@[]") }
	switch p.Type {
	case "icmp":
		if !hostOK(p.Target) || (strings.Contains(p.Target, ":") && net.ParseIP(p.Target) == nil) {
			return fmt.Errorf("ICMP 地址请填写 IP 或域名，不要包含协议或端口")
		}
	case "tcp":
		host, port, err := net.SplitHostPort(p.Target)
		n, portErr := strconv.Atoi(port)
		if err != nil || !hostOK(host) || portErr != nil || n < 1 || n > 65535 {
			return fmt.Errorf("TCP 地址请使用 域名:端口、IPv4:端口 或 [IPv6]:端口")
		}
	case "http":
		u, err := url.Parse(p.Target)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || !hostOK(u.Hostname()) || u.User != nil {
			return fmt.Errorf("HTTP 地址请填写完整的 http:// 或 https:// 网址，不要包含登录凭据")
		}
		if port := u.Port(); port != "" {
			n, err := strconv.Atoi(port)
			if err != nil || n < 1 || n > 65535 {
				return fmt.Errorf("HTTP 端口必须在 1 至 65535 之间")
			}
		}
	default:
		return fmt.Errorf("探测方式仅支持 ICMP、TCP 或 HTTP")
	}
	return nil
}

func ImportPingPresets(ids, clients []string, defaultOn bool, interval int, overrides ...PingPresetOverride) (PingPresetImportResult, error) {
	pingPresetImportMu.Lock()
	defer pingPresetImportMu.Unlock()
	result, err := importPingPresets(dbcore.GetDBInstance(), ids, clients, defaultOn, interval, overrides...)
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
func importPingPresets(db *gorm.DB, ids, clients []string, defaultOn bool, interval int, overrides ...PingPresetOverride) (PingPresetImportResult, error) {
	result := PingPresetImportResult{TaskIDs: []uint{}}
	if len(ids) == 0 || len(ids) > 256 {
		return result, fmt.Errorf("请选择 1 至 256 个预设")
	}
	if len(overrides) > len(ids) {
		return result, fmt.Errorf("自定义项不能多于所选预设")
	}
	edits := map[string]PingPresetOverride{}
	for _, edit := range overrides {
		if _, duplicate := edits[edit.ID]; duplicate {
			return result, fmt.Errorf("自定义预设不能重复：%s", edit.ID)
		}
		edit.Name = strings.TrimSpace(edit.Name)
		edit.Target = strings.TrimSpace(edit.Target)
		if err := validatePingPresetOverride(edit); err != nil {
			return result, err
		}
		edits[edit.ID] = edit
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
			if edit, changed := edits[id]; changed {
				p.Name, p.Type, p.Target = edit.Name, edit.Type, edit.Target
			}
			selected = append(selected, p)
			seen[id] = true
		}
	}
	for id := range edits {
		if !seen[id] {
			return result, fmt.Errorf("只能编辑已选中的预设：%s", id)
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
		lookup := map[string]*models.PingTask{}
		for i := range existing {
			k := pingPresetTargetKey(existing[i].Type, existing[i].Target)
			if lookup[k] == nil {
				lookup[k] = &existing[i]
			}
		}
		for _, p := range selected {
			if old := lookup[pingPresetTargetKey(p.Type, p.Target)]; old != nil {
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
			lookup[pingPresetTargetKey(p.Type, p.Target)] = &task
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
