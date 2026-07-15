//go:build windows

package beta

import (
	"os/exec"
	"runtime"
	"strings"
)

func getOSVersion() string {
	out, err := exec.Command("cmd", "/c", "ver").Output()
	if err == nil {
		return "Windows " + strings.TrimSpace(string(out))
	}
	return "Windows (" + runtime.GOARCH + ")"
}
