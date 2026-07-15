//go:build windows

package beta

import (
	"os/exec"
	"strings"
)

func getBatteryInfo() string {
	out, err := exec.Command("WMIC", "PATH", "Win32_Battery", "Get", "EstimatedChargeRemaining").Output()
	if err != nil {
		out2, err2 := exec.Command("powershell", "-Command", "(Get-WmiObject Win32_Battery).EstimatedChargeRemaining").Output()
		if err2 != nil {
			return "unknown"
		}
		return strings.TrimSpace(string(out2)) + "%"
	}
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line != "" && line != "EstimatedChargeRemaining" {
			return line + "%"
		}
	}
	return "unknown"
}
