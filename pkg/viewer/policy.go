// Package viewer defines the data that may cross the read-only audience boundary.
package viewer

import (
	"encoding/json"
	"github.com/komari-monitor/komari/database/dbcore"
	"github.com/komari-monitor/komari/database/models"
	"github.com/komari-monitor/komari/pkg/config"
	"strings"
)

type Policy struct{ Price, Expiry bool }

func CurrentPolicy() Policy {
	theme, err := config.GetAs[string](config.ThemeKey)
	if err != nil {
		return Policy{}
	}
	var row models.ThemeConfiguration
	if dbcore.GetDBInstance().Where("short = ?", theme).First(&row).Error != nil {
		return Policy{}
	}
	var values map[string]any
	if json.Unmarshal([]byte(row.Data), &values) != nil {
		return Policy{}
	}
	return PolicyFromTheme(values)
}
func PolicyFromTheme(values map[string]any) Policy {
	flags, _ := values["guestDisplay"].(map[string]any)
	price, _ := flags["showPrice"].(bool)
	expiry, _ := flags["showExpiredAt"].(bool)
	if v, ok := values["guestDisplay.showPrice"].(bool); ok {
		price = v
	}
	if v, ok := values["guestDisplay.showExpiredAt"].(bool); ok {
		expiry = v
	}
	return Policy{price, expiry}
}

func pick(input map[string]any, keys string) map[string]any {
	out := map[string]any{}
	for _, key := range strings.Fields(keys) {
		if value, ok := input[key]; ok {
			out[key] = value
		}
	}
	return out
}

// PublicTheme is an explicit schema, including nested schemas. Unknown keys,
// onboarding state and plugin/admin state never reach a viewer.
func PublicTheme(input map[string]any) map[string]any {
	out := pick(input, "colorTheme cardLayout cardDesign statusDesign graphDesign showRamDiskTotal backgroundImageUrl backgroundImageUrlDesktop backgroundImageUrlMobile mainContentWidth offlineServerPosition showServerListInDetails backgroundBlurEnabled backgroundBlurType backgroundBlurIntensity backgroundMaskOpacity cardBlurEnabled cardBlurType cardBlurIntensity cardExtraBlurIntensity nodeViewMode appearance language")
	nested := map[string]string{
		"guestDisplay":          "showPrice showExpiredAt",
		"statusCardsVisibility": "currentTime currentOnline regionOverview trafficOverview networkSpeed",
	}
	for key, keys := range nested {
		if obj, ok := input[key].(map[string]any); ok {
			out[key] = pick(obj, keys)
		}
	}
	for _, key := range []string{"guestDisplay.showPrice", "guestDisplay.showExpiredAt"} {
		if v, ok := input[key].(bool); ok {
			out[key] = v
		}
	}
	return out
}
func PublicSettings(input map[string]any) map[string]any {
	out := pick(input, "sitename description record_enabled theme")
	encoded, _ := json.Marshal(input["theme_settings"])
	var theme map[string]any
	_ = json.Unmarshal(encoded, &theme)
	out["theme_settings"] = PublicTheme(theme)
	out["private_site"] = false
	out["is_share_view"] = true
	return out
}

// Node returns no database model. Adding a database column cannot make it public.
func Node(node models.Client, policy Policy) map[string]any {
	out := map[string]any{
		"uuid": node.UUID, "name": node.Name, "cpu_name": node.CpuName,
		"virtualization": node.Virtualization, "arch": node.Arch, "cpu_cores": node.CpuCores,
		"cpu_physical_cores": node.CpuPhysicalCores, "os": node.OS, "gpu_name": node.GpuName,
		"region": node.Region, "public_remark": node.PublicRemark,
		"mem_total": node.MemTotal, "swap_total": node.SwapTotal, "disk_total": node.DiskTotal,
		"traffic_limit": node.TrafficLimit, "traffic_limit_type": node.TrafficLimitType,
	}
	if policy.Price {
		out["price"] = node.Price
		out["currency"] = node.Currency
		out["billing_cycle"] = node.BillingCycle
	}
	if policy.Expiry {
		out["expired_at"] = node.ExpiredAt
	}
	return out
}
