package beta

import (
	"bytes"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
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

func isCDCommand(cmd string) bool {
	trimmed := strings.TrimSpace(cmd)
	if trimmed == "cd" || trimmed == "chdir" {
		return true
	}
	if strings.HasPrefix(trimmed, "cd ") || strings.HasPrefix(trimmed, "chdir ") {
		return true
	}
	if runtime.GOOS == "windows" {
		if strings.HasPrefix(trimmed, "cd/") || strings.HasPrefix(trimmed, "chdir/") {
			return true
		}
	}
	return false
}

func (s *Session) HandleExec(cmd, stdin string, onStdout func([]byte), onStderr func([]byte)) (int, string, error) {
	trimmed := strings.TrimSpace(cmd)
	if isCDCommand(trimmed) {
		newCwd, err := s.HandleCD(trimmed)
		if err != nil {
			if onStderr != nil {
				onStderr([]byte(err.Error() + "\n"))
			}
			return 1, s.GetCwd(), nil
		}
		return 0, newCwd, nil
	}

	shell := s.GetShell()
	cwd := s.GetCwd()

	command, err := buildCommand(shell, cmd)
	if err != nil {
		return -1, cwd, fmt.Errorf("build command: %w", err)
	}
	command.Dir = cwd

	if stdin != "" {
		command.Stdin = strings.NewReader(stdin)
	}

	stdout, err := command.StdoutPipe()
	if err != nil {
		return -1, cwd, fmt.Errorf("stdout pipe: %w", err)
	}
	stderr, err := command.StderrPipe()
	if err != nil {
		return -1, cwd, fmt.Errorf("stderr pipe: %w", err)
	}

	if err := command.Start(); err != nil {
		return -1, cwd, fmt.Errorf("start: %w", err)
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

	newCwd := s.GetCwd()
	return code, newCwd, nil
}

func ExecuteSimple(shell, cmd, cwd string, stdin []byte) ([]byte, []byte, int, error) {
	command, err := buildCommand(shell, cmd)
	if err != nil {
		return nil, nil, -1, err
	}
	command.Dir = cwd
	if stdin != nil {
		command.Stdin = bytes.NewReader(stdin)
	}

	var stdout, stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr

	if err := command.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return stdout.Bytes(), stderr.Bytes(), exitErr.ExitCode(), nil
		}
		return stdout.Bytes(), stderr.Bytes(), -1, err
	}
	return stdout.Bytes(), stderr.Bytes(), 0, nil
}
