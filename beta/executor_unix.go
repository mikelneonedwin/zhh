//go:build !windows
// +build !windows

package beta

import (
	"fmt"
	"os/exec"

	"github.com/creack/pty"
)

func buildCommand(shell, cmd string) (*exec.Cmd, error) {
	switch shell {
	case "cmd":
		return exec.Command("cmd", "/c", cmd), nil
	case "powershell":
		return exec.Command("powershell", "-NoProfile", "-Command", cmd), nil
	case "pwsh":
		return exec.Command("pwsh", "-NoProfile", "-Command", cmd), nil
	default:
		return exec.Command(shell, "-c", cmd), nil
	}
}

func runCommandPlatform(command *exec.Cmd, stdin []byte, onStdout, onStderr func([]byte)) (int, error) {
	f, err := pty.Start(command)
	if err != nil {
		return -1, fmt.Errorf("pty start: %w", err)
	}
	defer f.Close()

	if len(stdin) > 0 {
		f.Write(stdin)
	}

	stdoutDone := make(chan struct{})
	go func() {
		buf := make([]byte, 32768)
		for {
			n, err := f.Read(buf)
			if n > 0 {
				data := make([]byte, n)
				copy(data, buf[:n])
				if onStdout != nil {
					onStdout(data)
				}
			}
			if err != nil {
				break
			}
		}
		close(stdoutDone)
	}()

	<-stdoutDone

	code := 0
	if err := command.Wait(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			code = exitErr.ExitCode()
		} else {
			code = -1
		}
	}
	return code, nil
}
