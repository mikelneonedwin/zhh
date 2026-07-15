//go:build windows
// +build windows

package beta

import (
	"bytes"
	"fmt"
	"os/exec"
	"syscall"
)

func buildCommand(shell, cmd string) (*exec.Cmd, error) {
	switch shell {
	case "cmd":
		// On Windows cmd.exe, we must set SysProcAttr.CmdLine to bypass Go's automatic escaping.
		// We format it as: cmd.exe /c "our command"
		// This preserves the inner quotes perfectly because cmd.exe strips the outer quotes.
		c := exec.Command("cmd")
		c.SysProcAttr = &syscall.SysProcAttr{
			CmdLine: `/c "` + cmd + `"`,
		}
		return c, nil
	case "powershell":
		return exec.Command("powershell", "-NoProfile", "-Command", cmd), nil
	case "pwsh":
		return exec.Command("pwsh", "-NoProfile", "-Command", cmd), nil
	default:
		return exec.Command(shell, "-c", cmd), nil
	}
}

func runCommandPlatform(command *exec.Cmd, stdin []byte, onStdout, onStderr func([]byte)) (int, error) {
	if len(stdin) > 0 {
		command.Stdin = bytes.NewReader(stdin)
	}

	stdout, err := command.StdoutPipe()
	if err != nil {
		return -1, fmt.Errorf("stdout pipe: %w", err)
	}
	stderr, err := command.StderrPipe()
	if err != nil {
		return -1, fmt.Errorf("stderr pipe: %w", err)
	}

	if err := command.Start(); err != nil {
		return -1, fmt.Errorf("start: %w", err)
	}

	stdoutDone := make(chan struct{})
	go func() {
		buf := make([]byte, 32768)
		for {
			n, err := stdout.Read(buf)
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

	stderrDone := make(chan struct{})
	go func() {
		buf := make([]byte, 32768)
		for {
			n, err := stderr.Read(buf)
			if n > 0 {
				data := make([]byte, n)
				copy(data, buf[:n])
				if onStderr != nil {
					onStderr(data)
				}
			}
			if err != nil {
				break
			}
		}
		close(stderrDone)
	}()

	<-stdoutDone
	<-stderrDone

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
