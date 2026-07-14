//go:build darwin

package beta

import (
	"os/exec"
	"strings"
)

func getBatteryInfo() string {
	out, err := exec.Command("pmset", "-g", "batt").Output()
	if err != nil {
		return "unknown"
	}
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if strings.Contains(line, "%") {
			fields := strings.Fields(line)
			for _, f := range fields {
				if strings.HasSuffix(f, "%") {
					return f
				}
			}
		}
	}
	return "unknown"
}
