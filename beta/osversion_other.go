//go:build !linux && !darwin && !windows

package beta

import "runtime"

func getOSVersion() string {
	return runtime.GOOS
}
