//go:build linux

package beta

import (
	"fmt"
	"syscall"
)

func getStorageInfo() string {
	var stat syscall.Statfs_t
	if err := syscall.Statfs("/", &stat); err != nil {
		return "unknown"
	}
	total := stat.Blocks * uint64(stat.Bsize)
	free := stat.Bfree * uint64(stat.Bsize)
	used := total - free
	if total == 0 {
		return "unknown"
	}
	return fmt.Sprintf("%.1f GB / %.1f GB used", float64(used)/1e9, float64(total)/1e9)
}
