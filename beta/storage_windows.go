//go:build windows

package beta

import (
	"fmt"
	"syscall"
)

func getStorageInfo() string {
	path, err := syscall.UTF16PtrFromString("C:\\")
	if err != nil {
		return "unknown"
	}
	var free, total, totalFree int64
	if err := syscall.GetDiskFreeSpaceEx(path, &free, &total, &totalFree); err != nil {
		return "unknown"
	}
	used := total - free
	return fmt.Sprintf("%.1f GB / %.1f GB used", float64(used)/1e9, float64(total)/1e9)
}
