package beta

import (
	"bytes"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
)



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

func (s *Session) HandleExec(cmd string, stdin []byte, onStdout func([]byte), onStderr func([]byte)) (int, string, error) {
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

	code, err := runCommandPlatform(command, stdin, onStdout, onStderr)
	if err != nil {
		return -1, cwd, err
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
