package alpha

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net"
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

	if ip, cmdRest, ok := parseIPPrefix(rest); ok {
		cmd := strings.TrimSpace(cmdRest)
		cmd = strings.TrimPrefix(cmd, ":")
		return stage{target: ip, cmd: strings.TrimSpace(cmd)}
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

	if strings.HasPrefix(rest, "alpha ") {
		cmd := strings.TrimPrefix(rest, "alpha ")
		return stage{target: "alpha", cmd: strings.TrimSpace(cmd)}
	} else if rest == "alpha" {
		return stage{target: "alpha", cmd: ""}
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
	// disabled as it is handled by pipelinePainter now
}

type pipelinePainter struct{}

func (p *pipelinePainter) Paint(line []rune, pos int) []rune {
	s := string(line)
	trimmed := strings.TrimSpace(s)
	if strings.HasPrefix(trimmed, "@cp ") || strings.HasPrefix(trimmed, "@copy ") ||
		strings.HasPrefix(trimmed, "@mv ") || strings.HasPrefix(trimmed, "@move ") {
		return []rune(colorizeTransferCommandLine(s))
	}

	var out []byte

	start := 0
	var quote rune
	subshell := 0

	for i, c := range s {
		switch {
		case quote == 0 && (c == '\'' || c == '"'):
			quote = c
		case quote != 0 && c == quote:
			quote = 0
		case quote == 0 && c == '(' && i > 0 && s[i-1] == '$':
			subshell++
		case quote == 0 && c == ')':
			if subshell > 0 {
				subshell--
			}
		case quote == 0 && subshell == 0 && c == '|':
			stageStr := s[start:i]
			out = append(out, colorizeStage(stageStr)...)
			out = append(out, '|')
			start = i + 1
		}
	}
	if start < len(s) {
		out = append(out, colorizeStage(s[start:])...)
	}

	return []rune(string(out))
}

type token struct {
	text    string
	start   int
	end     int
	isSpace bool
}

func tokenize(s string) []token {
	var tokens []token
	var current []rune
	start := 0
	inSpace := false

	runes := []rune(s)
	for i, r := range runes {
		isSpace := (r == ' ' || r == '\t' || r == '\n' || r == '\r')
		if i == 0 {
			inSpace = isSpace
		}
		if isSpace != inSpace {
			tokens = append(tokens, token{
				text:    string(current),
				start:   start,
				end:     i,
				isSpace: inSpace,
			})
			current = nil
			start = i
			inSpace = isSpace
		}
		current = append(current, r)
	}
	if len(current) > 0 {
		tokens = append(tokens, token{
			text:    string(current),
			start:   start,
			end:     len(runes),
			isSpace: inSpace,
		})
	}
	return tokens
}

func extractTarget(tok string) string {
	if tok == "$" {
		return "alpha"
	}
	rest := tok[1:]
	if idx := strings.Index(rest, ":"); idx != -1 {
		rest = rest[:idx]
	}
	if idx := strings.Index(rest, "/"); idx != -1 {
		rest = rest[:idx]
	}
	return rest
}

func colorizeTransferCommandLine(s string) string {
	tokens := tokenize(s)
	var out strings.Builder
	activeColor := ""

	for _, tok := range tokens {
		if tok.isSpace {
			out.WriteString(tok.text)
			continue
		}

		if strings.HasPrefix(tok.text, "$") {
			target := extractTarget(tok.text)
			color := getDeviceColor(target)
			if color != "" {
				out.WriteString(color + tok.text + resetCode)
				if !strings.Contains(tok.text, ":") && !strings.Contains(tok.text, "/") {
					activeColor = color
				} else {
					activeColor = ""
				}
			} else {
				out.WriteString(tok.text)
				activeColor = ""
			}
		} else {
			if activeColor != "" {
				out.WriteString(activeColor + tok.text + resetCode)
				activeColor = ""
			} else {
				out.WriteString(tok.text)
			}
		}
	}
	return out.String()
}

func colorizeStage(stageStr string) []byte {
	st := parseStage(stageStr)
	color := getDeviceColor(st.target)
	if color == "" {
		return []byte(stageStr)
	}
	return []byte(color + stageStr + resetCode)
}

func (a *Alpha) executePipeline(line string) error {
	stages := parseStages(line)
	if len(stages) == 0 {
		return nil
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
			_, err := runLocalCommand(rs.cmd, nil, false)
			return err
		}
		return executeSimpleBeta(rs.session, rs.cmd, nil)
	}

	var prevOutput []byte
	for i, rs := range resolved {
		isLast := (i == len(resolved)-1)
		if rs.session == nil {
			out, err := runLocalCommand(rs.cmd, prevOutput, !isLast)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Alpha stage error: %v\n", err)
				return err
			}
			prevOutput = out
		} else {
			if isLast {
				err := executeSimpleBeta(rs.session, rs.cmd, prevOutput)
				if err != nil {
					return err
				}
			} else {
				out, err := executeBetaCapture(rs.session, rs.cmd, prevOutput)
				if err != nil {
					return err
				}
				prevOutput = out
			}
		}
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
	if err := session.Send(protocol.NewMessage(protocol.MsgExec, &protocol.ExecPayload{
		Cmd:   cmd,
		Stdin: stdin,
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
	if err := session.Send(protocol.NewMessage(protocol.MsgExec, &protocol.ExecPayload{
		Cmd:   cmd,
		Stdin: stdin,
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

func runLocalCommand(cmd string, stdin []byte, captureOutput bool) ([]byte, error) {
	var command *exec.Cmd
	if len(stdin) > 0 {
		command = exec.Command(Shell(), "-c", cmd)
		command.Stdin = bytes.NewReader(stdin)
	} else {
		command = exec.Command(Shell(), "-c", cmd)
	}
	if !captureOutput {
		command.Stdout = os.Stdout
		command.Stderr = os.Stderr
		err := command.Run()
		return nil, err
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

func parseIPPrefix(s string) (string, string, bool) {
	maxLen := 45
	if len(s) < maxLen {
		maxLen = len(s)
	}
	for i := maxLen; i >= 4; i-- {
		prefix := s[:i]
		if net.ParseIP(prefix) != nil {
			if i == len(s) {
				return prefix, "", true
			}
			next := s[i]
			if next == ' ' || next == '/' || next == ':' || next == '\t' || next == '\n' || next == '\r' {
				return prefix, s[i:], true
			}
		}
	}
	return "", "", false
}
