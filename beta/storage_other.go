//go:build !linux && !darwin && !windows

package beta

func getStorageInfo() string {
	return "N/A"
}
