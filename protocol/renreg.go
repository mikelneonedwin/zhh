package protocol

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
)

func PerformRenreg(dir, pattern, replacement string) ([]string, error) {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, fmt.Errorf("invalid regex: %w", err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read dir: %w", err)
	}

	var renamed []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if re.MatchString(name) {
			newName := re.ReplaceAllString(name, replacement)
			if newName != name {
				oldPath := filepath.Join(dir, name)
				newPath := filepath.Join(dir, newName)
				if err := os.Rename(oldPath, newPath); err != nil {
					return renamed, fmt.Errorf("rename %s to %s: %w", name, newName, err)
				}
				renamed = append(renamed, fmt.Sprintf("%s -> %s", name, newName))
			}
		}
	}
	return renamed, nil
}
