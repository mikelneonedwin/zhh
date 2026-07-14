package alpha

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"unicode"

	"zhh/protocol"
)

type stage struct {
	target string // "active", "alpha", "2", ".2", etc.
	cmd    string
}

func parseStages(line string) []stage {
	parts := splitByPipe(line)
	var stages []stage
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		stages = append(stages, parseStage(part))
	}
	return stages
}

func splitByPipe(line string) []string {
	var parts []string
	start := 0
	var quote rune
	subshell := 0

	for i, c := range line {
		switch {
		case quote == 0 && (c == '\'' || c == '"'):
			quote = c
		case quote != 0 && c == quote:
			quote = 0
		case quote == 0 && c == '(' && i > 0 && line[i-1] == '$':
			subshell++
		case quote == 0 && c == ')':
			if subshell > 0 {
				subshell--
			}
		case quote == 0 && subshell == 0 && c == '|':
			parts = append(parts, line[start:i])
			start = i + 1
		}
	}
	if start < len(line) {
		parts = append(parts, line[start:])
	}
	return parts
}

func parseStage(s string) stage {
	s = strings.TrimSpace(s)
	if s == "" {
		return stage{target: "active", cmd: ""}
	}

	if !strings.HasPrefix(s, "$") {
		return stage{target: "active", cmd: s}
	}

	rest := s[1:]
	rest = strings.TrimSpace(rest)

	if rest == "" {
		return stage{target: "alpha", cmd: ""}
	}

	first := rest[0]

	if first == '.' {
		rest = rest[1:]
		numStr := extractDigits(rest)
		if numStr == "" {
			return stage{target: "alpha", cmd: rest}
		}
		target := "." + numStr
		cmd := rest[len(numStr):]
		cmd = strings.TrimSpace(cmd)
		cmd = strings.TrimPrefix(cmd, ":")
		return stage{target: target, cmd: strings.TrimSpace(cmd)}
	}

	if unicode.IsDigit(rune(first)) {
		numStr := extractDigits(rest)
		cmd := rest[len(numStr):]
		cmd = strings.TrimSpace(cmd)
		cmd = strings.TrimPrefix(cmd, ":")
		return stage{target: numStr, cmd: strings.TrimSpace(cmd)}
	}

	return stage{target: "alpha", cmd: rest}
}

func extractDigits(s string) string {
	for i, c := range s {
		if !unicode.IsDigit(c) {
			return s[:i]
		}
	}
	return s
}

func displayPipeline(stages []stage) {
	for i, st := range stages {
		if i > 0 {
			fmt.Print(" | ")
		}
		color := getDeviceColor(st.target)
		if color != "" {
			fmt.Print(color)
		}
		if st.target != "" && st.target != "active" {
			fmt.Print("$" + st.target + " ")
		}
		fmt.Print(st.cmd)
		if color != "" {
			fmt.Print(resetCode)
		}
	}
	fmt.Println()
}

func (a *Alpha) executePipeline(line string) error {
	stages := parseStages(line)
	if len(stages) == 0 {
		return nil
	}

	if len(stages) > 1 || stages[0].target != "active" {
		displayPipeline(stages)
	}

	type rs struct {
		session *BetaSession
		cmd     string
	}
	var resolved []rs

	for _, st := range stages {
		sess, err := a.resolveStageTarget(st.target)
		if err != nil {
			return err
		}
		resolved = append(resolved, rs{session: sess, cmd: st.cmd})
	}

	if len(resolved) == 1 {
		rs := resolved[0]
		if rs.session == nil {
			out, err := runLocalCommand(rs.cmd, nil)
			if err == nil && len(out) > 0 {
				os.Stdout.Write(out)
			}
			return err
		}
		return executeSimpleBeta(rs.session, rs.cmd, nil)
	}

	var prevOutput []byte
	for _, rs := range resolved {
		if rs.session == nil {
			out, err := runLocalCommand(rs.cmd, prevOutput)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Alpha stage error: %v\n", err)
				return err
			}
			prevOutput = out
		} else {
			out, err := executeBetaCapture(rs.session, rs.cmd, prevOutput)
			if err != nil {
				return err
			}
			prevOutput = out
		}
	}

	if len(prevOutput) > 0 {
		os.Stdout.Write(prevOutput)
	}
	return nil
}

func (a *Alpha) resolveStageTarget(target string) (*BetaSession, error) {
	switch target {
	case "", "active":
		return a.ActiveSession(), nil
	case "alpha":
		return nil, nil
	default:
		return a.ResolveDevice(target)
	}
}

func executeSimpleBeta(session *BetaSession, cmd string, stdin []byte) error {
	var stdinStr string
	if len(stdin) > 0 {
		stdinStr = string(stdin)
	}

	if err := session.Send(protocol.NewMessage(protocol.MsgExec, &protocol.ExecPayload{
		Cmd:   cmd,
		Stdin: stdinStr,
	})); err != nil {
		return fmt.Errorf("send exec: %w", err)
	}

	for {
		msg, err := session.Read()
		if err != nil {
			return fmt.Errorf("read: %w", err)
		}

		switch msg.Type {
		case protocol.MsgExecStdout:
			var p protocol.ExecOutputPayload
			json.Unmarshal(msg.Payload, &p)
			os.Stdout.Write(p.Data)
		case protocol.MsgExecStderr:
			var p protocol.ExecOutputPayload
			json.Unmarshal(msg.Payload, &p)
			os.Stderr.Write(p.Data)
		case protocol.MsgExecDone:
			var p protocol.ExecDonePayload
			json.Unmarshal(msg.Payload, &p)
			session.mu.Lock()
			if p.Cwd != "" {
				session.Cwd = p.Cwd
			}
			session.mu.Unlock()
			return nil
		case protocol.MsgError:
			var p protocol.ErrorPayload
			json.Unmarshal(msg.Payload, &p)
			return fmt.Errorf("beta error: %s", p.Message)
		}
	}
}

func executeBetaCapture(session *BetaSession, cmd string, stdin []byte) ([]byte, error) {
	var stdinStr string
	if len(stdin) > 0 {
		stdinStr = string(stdin)
	}

	if err := session.Send(protocol.NewMessage(protocol.MsgExec, &protocol.ExecPayload{
		Cmd:   cmd,
		Stdin: stdinStr,
	})); err != nil {
		return nil, fmt.Errorf("send exec: %w", err)
	}

	var output bytes.Buffer
	for {
		msg, err := session.Read()
		if err != nil {
			return nil, fmt.Errorf("read: %w", err)
		}

		switch msg.Type {
		case protocol.MsgExecStdout:
			var p protocol.ExecOutputPayload
			json.Unmarshal(msg.Payload, &p)
			output.Write(p.Data)
		case protocol.MsgExecStderr:
			var p protocol.ExecOutputPayload
			json.Unmarshal(msg.Payload, &p)
			os.Stderr.Write(p.Data)
		case protocol.MsgExecDone:
			var p protocol.ExecDonePayload
			json.Unmarshal(msg.Payload, &p)
			session.mu.Lock()
			if p.Cwd != "" {
				session.Cwd = p.Cwd
			}
			session.mu.Unlock()
			return output.Bytes(), nil
		case protocol.MsgError:
			var p protocol.ErrorPayload
			json.Unmarshal(msg.Payload, &p)
			return output.Bytes(), fmt.Errorf("beta error: %s", p.Message)
		}
	}
}

func runLocalCommand(cmd string, stdin []byte) ([]byte, error) {
	var command *exec.Cmd
	if len(stdin) > 0 {
		command = exec.Command(Shell(), "-c", cmd)
		command.Stdin = bytes.NewReader(stdin)
	} else {
		command = exec.Command(Shell(), "-c", cmd)
	}
	return command.Output()
}

func Shell() string {
	sh := os.Getenv("SHELL")
	if sh != "" {
		return sh
	}
	return "sh"
}
