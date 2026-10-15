package geoip

import (
	"log/slog"
	"net"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/komari-monitor/komari/pkg/config"
	"github.com/patrickmn/go-cache"
)

var CurrentProvider GeoIPService
var geoCache *cache.Cache
var providerMu sync.RWMutex
var initMu sync.Mutex

type GeoInfo struct {
	ISOCode string
	Name    string
}

func init() {
	CurrentProvider = &EmptyProvider{}
	geoCache = cache.New(48*time.Hour, 1*time.Hour)
}

// GeoIPService 接口定义了获取地理位置信息的核心方法。
// 任何实现此接口的类型都可以作为地理位置服务提供者。
type GeoIPService interface {
	Name() string

	GetGeoInfo(ip net.IP) (*GeoInfo, error)

	UpdateDatabase() error

	Close() error
}

func GetRegionUnicodeEmoji(isoCode string) string {
	if len(isoCode) != 2 {
		return ""
	}
	isoCode = strings.ToUpper(isoCode)

	if !unicode.IsLetter(rune(isoCode[0])) || !unicode.IsLetter(rune(isoCode[1])) {
		return ""
	}

	rune1 := rune(0x1F1E6 + (rune(isoCode[0]) - 'A'))
	rune2 := rune(0x1F1E6 + (rune(isoCode[1]) - 'A'))
	return string(rune1) + string(rune2)
}

func InitGeoIp() {
	initMu.Lock()
	defer initMu.Unlock()
	conf, err := config.GetMany(map[string]any{
		config.GeoIpEnabledKey:  true,
		config.GeoIpProviderKey: "ipinfo",
	})
	if err != nil {
		panic("Failed to get configuration for GeoIP: " + err.Error())
	}
	if !conf[config.GeoIpEnabledKey].(bool) {
		replaceProvider(&EmptyProvider{})
		return
	}
	switch conf[config.GeoIpProviderKey].(string) {
	case "mmdb":
		NewCurrentProvider, err := NewMaxMindGeoIPService()
		if err != nil {
			slog.Error("Failed to initialize MaxMind GeoIP service: " + err.Error())
		}
		if NewCurrentProvider != nil {
			replaceProvider(NewCurrentProvider)
		} else {
			replaceProvider(&EmptyProvider{})
			slog.Error("Failed to initialize MaxMind GeoIP service, using EmptyProvider instead.")
		}
	case "ip-api":
		NewCurrentProvider, err := NewIPAPIService()
		if err != nil {
			slog.Error("Failed to initialize ip-api service: " + err.Error())
		}
		if NewCurrentProvider != nil {
			replaceProvider(NewCurrentProvider)
			slog.Info("Using ip-api.com as GeoIP provider.")
		} else {
			replaceProvider(&EmptyProvider{})
			slog.Info("Failed to initialize ip-api service, using EmptyProvider instead.")
		}
	case "geojs":
		NewCurrentProvider, err := NewGeoJSService()
		if err != nil {
			slog.Error("Failed to initialize GeoJS service: " + err.Error())
		}
		if NewCurrentProvider != nil {
			replaceProvider(NewCurrentProvider)
			slog.Info("Using geojs.io as GeoIP provider.")
		} else {
			replaceProvider(&EmptyProvider{})
			slog.Info("Failed to initialize GeoJS service, using EmptyProvider instead.")
		}
	case "ipinfo":
		NewCurrentProvider, err := NewIPInfoService()
		if err != nil {
			slog.Error("Failed to initialize IPInfo service: " + err.Error())
		}
		if NewCurrentProvider != nil {
			replaceProvider(NewCurrentProvider)
			slog.Info("Using ipinfo.io as GeoIP provider.")
		} else {
			replaceProvider(&EmptyProvider{})
			slog.Info("Failed to initialize IPInfo service, using EmptyProvider instead.")
		}
	default:
		replaceProvider(&EmptyProvider{})
	}
}

func GetGeoInfo(ip net.IP) (*GeoInfo, error) {
	providerMu.RLock()
	defer providerMu.RUnlock()
	providerName := CurrentProvider.Name()
	cacheKey := providerName + ":" + ip.String()

	if cachedInfo, found := geoCache.Get(cacheKey); found {
		//log.Println("GeoIP cache hit for", cacheKey)
		return cachedInfo.(*GeoInfo), nil
	}

	info, err := CurrentProvider.GetGeoInfo(ip)
	if err == nil && info != nil {
		//log.Println("GeoIP cache miss for", cacheKey)
		geoCache.Set(cacheKey, info, cache.DefaultExpiration)
	}
	return info, err
}

func UpdateDatabase() error {
	providerMu.RLock()
	defer providerMu.RUnlock()
	err := CurrentProvider.UpdateDatabase()
	if err == nil {
		geoCache.Flush()
		slog.Info("GeoIP cache cleared due to database update.")
	}
	return err
}

// ProviderName is safe to inspect while runtime settings are changing.
func ProviderName() string {
	providerMu.RLock()
	defer providerMu.RUnlock()
	return CurrentProvider.Name()
}

func replaceProvider(next GeoIPService) {
	providerMu.Lock()
	defer providerMu.Unlock()
	previous := CurrentProvider
	CurrentProvider = next
	if previous != nil && previous != next {
		_ = previous.Close()
	}
}
