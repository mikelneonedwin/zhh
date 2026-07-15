//go:build darwin

package beta

import (
	"os/exec"
	"strings"
)

func getOSVersion() string {
	out, err := exec.Command("sw_vers", "-productVersion").Output()
	if err != nil {
		return "macOS"
	}
	return "macOS " + strings.TrimSpace(string(out))
}
