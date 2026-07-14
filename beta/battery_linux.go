//go:build linux

package beta

import (
	"os"
	"path/filepath"
	"strings"
)

func getBatteryInfo() string {
	files, _ := filepath.Glob("/sys/class/power_supply/BAT*/capacity")
	if len(files) == 0 {
		return "No battery"
	}
	data, err := os.ReadFile(files[0])
	if err != nil {
		return "unknown"
	}
	return strings.TrimSpace(string(data)) + "%"
}
