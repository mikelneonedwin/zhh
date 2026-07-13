package beta

import (
	"os"
	"os/exec"
	"runtime"
	"strings"
)

func DetectShells() []string {
	var shells []string
	if runtime.GOOS == "windows" {
		shells = append(shells, "cmd")
		if _, err := exec.LookPath("powershell"); err == nil {
			shells = append(shells, "powershell")
		}
		if _, err := exec.LookPath("pwsh"); err == nil {
			shells = append(shells, "pwsh")
		}
	} else {
		for _, s := range []string{"bash", "zsh", "sh", "fish", "dash"} {
			if _, err := exec.LookPath(s); err == nil {
				shells = append(shells, s)
			}
		}
	}
	if len(shells) == 0 {
		if runtime.GOOS == "windows" {
			shells = append(shells, "cmd")
		} else {
			shells = append(shells, "sh")
		}
	}
	return shells
}

func DefaultShell() string {
	if runtime.GOOS == "windows" {
		return "cmd"
	}
	shell := os.Getenv("SHELL")
	if shell != "" {
		parts := strings.Split(shell, "/")
		return parts[len(parts)-1]
	}
	return "sh"
}

func ShellName(shell string) string {
	switch shell {
	case "cmd":
		return "cmd"
	case "powershell":
		return "powershell"
	case "pwsh":
		return "pwsh"
	case "bash":
		return "bash"
	case "zsh":
		return "zsh"
	case "fish":
		return "fish"
	default:
		return shell
	}
}
