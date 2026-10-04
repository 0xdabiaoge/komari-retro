package config

import (
	"crypto/rand"
	"strings"
	"time"
)

type Settings struct {
	ID                     uint   `json:"id,omitempty"`                                        // 1
	Sitename               string `json:"sitename" default:"Komari"`                           // 站点名称，默认 "Komari"
	Description            string `json:"description" default:"A simple server monitor tool."` // 站点描述
	CorsOriginCheckEnabled bool   `json:"cors_origin_check_enabled" default:"true"`            // 是否启用 API CORS 跨域请求校验，默认 true
	CorsAllowedOrigins     string `json:"cors_allowed_origins" default:""`                     // API 跨域允许列表
	WsOriginCheckEnabled   bool   `json:"ws_origin_check_enabled" default:"true"`              // 是否校验 WebSocket Origin
	WsAllowedOrigins       string `json:"ws_allowed_origins" default:""`                       // WebSocket Origin 允许列表
	Theme                  string `json:"theme" default:"default"`                             // 主题名称，默认 'default'
	PrivateSite            bool   `json:"private_site" default:"false"`                        // 是否为私有站点，默认 false
	ApiKey                 string `json:"api_key" default:""`                                  // API 密钥，默认空字符串
	AutoDiscoveryKey       string `json:"auto_discovery_key" default:""`                       // 自动发现密钥
	ScriptDomain           string `json:"script_domain" default:""`                            // 自定义脚本域名
	SendIpAddrToGuest      bool   `json:"send_ip_addr_to_guest" default:"false"`               // 是否向访客页面发送 IP 地址，默认 false
	EulaAccepted           bool   `json:"eula_accepted" default:"false"`
	BaseScriptsURLKey      string `json:"base_scripts_url" default:""`
	// GeoIP 配置
	GeoIpEnabled  bool   `json:"geo_ip_enabled" default:"true"`
	GeoIpProvider string `json:"geo_ip_provider" default:"ipinfo"` // empty, mmdb, ip-api, geojs
	// OAuth 配置
	OAuthEnabled          bool   `json:"o_auth_enabled" default:"false"`
	OAuthProvider         string `json:"o_auth_provider" default:"github"`
	DisablePasswordLogin  bool   `json:"disable_password_login" default:"false"`
	CloudflareTunnelToken string `json:"cloudflare_tunnel_token" default:""`
	// 自定义美化
	CustomHead string `json:"custom_head" default:""`
	CustomBody string `json:"custom_body" default:""`
	// 通知
	NotificationEnabled        bool    `json:"notification_enabled" default:"true"` // 通知总开关
	NotificationMethod         string  `json:"notification_method" default:"none"`
	NotificationTemplate       string  `json:"notification_template" default:"{{emoji}} <b>【Komari 监控告警】</b> {{emoji}}\n━━━━━━━━━━━━━━━\n📌 <b>告警事件</b>: <code>{{status}}</code>\n🖥 <b>监控节点</b>: <b>{{client}}</b>\n🌐 <b>节点网络</b>: <code>{{ip}}</code> ({{region}})\n📝 <b>告警详情</b>: {{message}}\n⏰ <b>告警时间</b>: <code>{{time}}</code>\n━━━━━━━━━━━━━━━\n🔔 <i>来自 {{site_name}} 监控平台</i>"`
	ExpireNotificationEnabled  bool    `json:"expire_notification_enabled" default:"true"` // 是否启用过期通知
	ExpireNotificationLeadDays int     `json:"expire_notification_lead_days" default:"7"`  // 过期前多少天通知，默认7天
	LoginNotification          bool    `json:"login_notification" default:"true"`          // 登录通知
	TrafficLimitPercentage     float64 `json:"traffic_limit_percentage" default:"80.00"`   // 流量限制百分比，默认80.00%
	// Record
	RecordEnabled          bool `json:"record_enabled" default:"true"`          // 是否启用记录功能
	RecordPreserveTime     int  `json:"record_preserve_time" default:"720"`     // 记录保留时间，单位小时，默认30天
	PingRecordPreserveTime int  `json:"ping_record_preserve_time" default:"24"` // Ping 记录保留时间，单位小时，默认1天
	AdminPath              string `json:"admin_path" default:""`                 // 后台安全入口路径（例如 /entry-xxxxxx）
	AdminViewPath          string `json:"admin_view_path" default:""`            // 管理员查看探针前台地址（例如 /view-xxxxxx）
	TemporyShareToken      string `json:"tempory_share_token" default:""`        // 临时只读分享密钥
	TemporyShareTokenExpireAt int64 `json:"tempory_share_token_expire_at" default:"0"` // 临时分享过期时间戳
	PermanentShareToken    string `json:"permanent_share_token" default:""`      // 永久只读分享密钥
	UpdatedAt              time.Time
}

const (
	SitenameKey                   = "sitename"
	DescriptionKey                = "description"
	CorsOriginCheckEnabledKey     = "cors_origin_check_enabled"
	CorsAllowedOriginsKey         = "cors_allowed_origins"
	WsOriginCheckEnabledKey       = "ws_origin_check_enabled"
	WsAllowedOriginsKey           = "ws_allowed_origins"
	ThemeKey                      = "theme"
	PrivateSiteKey                = "private_site"
	AdminPathKey                  = "admin_path"
	AdminViewPathKey              = "admin_view_path"
	TemporyShareTokenKey          = "tempory_share_token"
	TemporyShareTokenExpireAtKey  = "tempory_share_token_expire_at"
	PermanentShareTokenKey        = "permanent_share_token"
	ApiKeyKey                     = "api_key"
	AutoDiscoveryKeyKey           = "auto_discovery_key"
	ScriptDomainKey               = "script_domain"
	SendIpAddrToGuestKey          = "send_ip_addr_to_guest"
	EulaAcceptedKey               = "eula_accepted"
	BaseScriptsURLKey             = "base_scripts_url"
	GeoIpEnabledKey               = "geo_ip_enabled"
	GeoIpProviderKey              = "geo_ip_provider"
	OAuthEnabledKey               = "o_auth_enabled"
	OAuthProviderKey              = "o_auth_provider"
	DisablePasswordLoginKey       = "disable_password_login"
	CloudflareTunnelTokenKey      = "cloudflare_tunnel_token"
	CustomHeadKey                 = "custom_head"
	CustomBodyKey                 = "custom_body"
	NotificationEnabledKey        = "notification_enabled"
	NotificationMethodKey         = "notification_method"
	NotificationTemplateKey       = "notification_template"
	ExpireNotificationEnabledKey  = "expire_notification_enabled"
	ExpireNotificationLeadDaysKey = "expire_notification_lead_days"
	LoginNotificationKey          = "login_notification"
	TrafficLimitPercentageKey     = "traffic_limit_percentage"
	RecordEnabledKey              = "record_enabled"
	RecordPreserveTimeKey         = "record_preserve_time"
	PingRecordPreserveTimeKey     = "ping_record_preserve_time"
	UpdatedAtKey                  = "updated_at"
	XtermjsSettingsKey            = "xtermjs_settings"
)

// GetOrGenerateAdminPath 获取或自动生成后台安全入口路径（如 /entry-xxxxxx）
func GetOrGenerateAdminPath() string {
	path, _ := GetAs[string](AdminPathKey, "")
	path = strings.TrimSpace(path)
	if path != "" {
		if !strings.HasPrefix(path, "/") {
			path = "/" + path
		}
		return path
	}
	const chars = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, 6)
	_, _ = rand.Read(b)
	for i := range b {
		b[i] = chars[int(b[i])%len(chars)]
	}
	newPath := "/entry-" + string(b)
	_ = Set(AdminPathKey, newPath)
	return newPath
}

// GetOrGenerateAdminViewPath 获取或自动生成管理员查看探针前台路径（如 /view-xxxxxx）
func GetOrGenerateAdminViewPath() string {
	path, _ := GetAs[string](AdminViewPathKey, "")
	path = strings.TrimSpace(path)
	if path != "" {
		if !strings.HasPrefix(path, "/") {
			path = "/" + path
		}
		return path
	}
	const chars = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, 6)
	_, _ = rand.Read(b)
	for i := range b {
		b[i] = chars[int(b[i])%len(chars)]
	}
	newPath := "/view-" + string(b)
	_ = Set(AdminViewPathKey, newPath)
	return newPath
}
