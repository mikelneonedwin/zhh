//go:build linux

package beta

import (
	"os"
	"strings"
)

func getOSVersion() string {
	data, err := os.ReadFile("/etc/os-release")
	if err == nil {
		for _, line := range strings.Split(string(data), "\n") {
			if strings.HasPrefix(line, "PRETTY_NAME=") {
				return strings.Trim(strings.TrimPrefix(line, "PRETTY_NAME="), "\" \n\r")
			}
		}
	}
	data, err = os.ReadFile("/proc/sys/kernel/osrelease")
	if err == nil {
		return "Linux " + strings.TrimSpace(string(data))
	}
	return "Linux"
}
