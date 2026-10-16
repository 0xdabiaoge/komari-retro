package cmd

import "strconv"

// Agent WebSockets occupy listener slots for their entire lifetime. Leave
// room for administration and guest traffic in a several-hundred-node fleet.
func serverConnectionLimit() int {
	value, err := strconv.Atoi(GetEnv("KOMARI_MAX_CONNECTIONS", "2048"))
	if err != nil || value < 256 || value > 8192 {
		return 2048
	}
	return value
}
