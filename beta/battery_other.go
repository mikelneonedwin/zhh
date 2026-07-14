//go:build !linux && !darwin && !windows

package beta

func getBatteryInfo() string {
	return "N/A"
}
