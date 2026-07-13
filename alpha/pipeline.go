package alpha

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"zhh/protocol"
)

type stage struct {
	runner string // "alpha" or "beta"
	cmd    string
}

func parsePipeline(line string) []stage {
	parts := splitByPipe(line)
	var stages []stage
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if strings.HasPrefix(part, "$$") {
			stages = append(stages, stage{runner: "alpha", cmd: strings.TrimPrefix(part, "$$")})
		} else {
			stages = append(stages, stage{runner: "beta", cmd: part})
		}
	}
	return stages
}

func splitByPipe(line string) []string {
	var parts []string
	depth := 0
	start := 0
	for i, c := range line {
		switch c {
		case '"', '\'':
			depth++
		case '|':
			if depth == 0 {
				parts = append(parts, line[start:i])
				start = i + 1
			}
		}
	}
	if start < len(line) {
		parts = append(parts, line[start:])
	}
	return parts
}

func executePipeline(alpha *Alpha, session *BetaSession, line string) error {
	stages := parsePipeline(line)
	if len(stages) == 0 {
		return nil
	}

	if len(stages) == 1 && stages[0].runner == "beta" {
		return executeSimpleBeta(session, stages[0].cmd, nil)
	}

	var prevOutput []byte

	for _, st := range stages {
		if st.runner == "alpha" {
			out, err := runLocalCommand(st.cmd, prevOutput)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Alpha stage error: %v\n", err)
				return err
			}
			prevOutput = out
		} else {
			out, err := executeBetaCapture(session, st.cmd, prevOutput)
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
