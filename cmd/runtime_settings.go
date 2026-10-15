package cmd

import (
	"github.com/komari-monitor/komari/pkg/config"
	"github.com/komari-monitor/komari/utils/geoip"
	"github.com/komari-monitor/komari/utils/messageSender"
)

func refreshRuntimeSettings(event config.ConfigEvent) {
	if event.IsChanged(config.GeoIpProviderKey) || event.IsChanged(config.GeoIpEnabledKey) {
		go geoip.InitGeoIp()
	}
	if event.IsChanged(config.NotificationMethodKey) {
		go messageSender.Initialize()
	}
}
