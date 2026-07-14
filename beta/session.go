package beta

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
)

type Session struct {
	mu     sync.Mutex
	Cwd    string
	Shell  string
	Shells []string
}

func NewSession() *Session {
	cwd, _ := os.UserHomeDir()
	if cwd == "" {
		cwd, _ = os.Getwd()
	}
	shells := DetectShells()
	return &Session{
		Cwd:    cwd,
		Shell:  DefaultShell(),
		Shells: shells,
	}
}

func (s *Session) GetCwd() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.Cwd
}

func (s *Session) SetCwd(dir string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Cwd = dir
}

func (s *Session) GetShell() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.Shell
}

func (s *Session) SetShell(shell string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, sh := range s.Shells {
		if sh == shell {
			s.Shell = shell
			return true
		}
	}
	return false
}

func (s *Session) HandleCD(cmd string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	cmd = strings.TrimSpace(cmd)
	parts := strings.Fields(cmd)
	if len(parts) == 0 {
		return s.Cwd, nil
	}

	var target string
	if len(parts) == 1 {
		home, err := os.UserHomeDir()
		if err != nil {
			return s.Cwd, err
		}
		target = home
	} else {
		target = parts[len(parts)-1]
	}

	if strings.HasPrefix(target, "~") {
		home, err := os.UserHomeDir()
		if err != nil {
			return s.Cwd, err
		}
		target = filepath.Join(home, target[1:])
	}

	if !filepath.IsAbs(target) {
		target = filepath.Join(s.Cwd, target)
	}

	target = filepath.Clean(target)

	info, err := os.Stat(target)
	if err != nil {
		return s.Cwd, fmt.Errorf("cd: %w", err)
	}
	if !info.IsDir() {
		return s.Cwd, fmt.Errorf("cd: %s is not a directory", target)
	}

	if runtime.GOOS == "windows" && len(target) >= 2 && target[1] == ':' {
		upper := strings.ToUpper(string(target[0]))
		target = upper + target[1:]
	}

	s.Cwd = target
	return s.Cwd, nil
}
