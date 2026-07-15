package alpha

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"zhh/protocol"
)

func parseTransferArgs(args []string) (srcDev, srcPath, dstDev, dstPath string, err error) {
	// Flatten combined $N:/path args
	var flat []string
	for _, arg := range args {
		dev, path := splitDevicePathArg(arg)
		flat = append(flat, dev)
		if path != "" {
			flat = append(flat, path)
		}
	}

	switch len(flat) {
	case 2:
		srcPath = flat[0]
		dstPath = flat[1]
	case 3:
		if strings.HasPrefix(flat[0], "$") {
			srcDev = flat[0]
			srcPath = flat[1]
			dstPath = flat[2]
		} else {
			srcPath = flat[0]
			dstDev = flat[1]
			dstPath = flat[2]
		}
	case 4:
		srcDev = flat[0]
		srcPath = flat[1]
		dstDev = flat[2]
		dstPath = flat[3]
	default:
		err = fmt.Errorf("invalid arguments: got %d, need [dev] <src> [dev] <dst>", len(args))
	}
	return
}

func splitDevicePathArg(arg string) (dev, path string) {
	if !strings.HasPrefix(arg, "$") {
		return "", arg
	}

	rest := arg[1:]

	if ip, pathRest, ok := parseIPPrefix(rest); ok {
		pathRest = strings.TrimSpace(pathRest)
		pathRest = strings.TrimPrefix(pathRest, ":")
		pathRest = strings.TrimSpace(pathRest)
		return "$" + ip, pathRest
	}

	if strings.HasPrefix(rest, ".") {
		rest = rest[1:]
		numStr := extractDigits(rest)
		if numStr == "" {
			return "", arg
		}
		dev = "$." + numStr
		rest = rest[len(numStr):]
	} else {
		numStr := extractDigits(rest)
		if numStr == "" {
			return "", arg
		}
		dev = "$" + numStr
		rest = rest[len(numStr):]
	}

	rest = strings.TrimSpace(rest)
	rest = strings.TrimPrefix(rest, ":")
	rest = strings.TrimSpace(rest)
	if rest != "" {
		path = rest
	}

	return
}

func handleFileTransfer(alpha *Alpha, srcDevSpec, srcPath, dstDevSpec, dstPath string, move bool) error {
	var srcSession, dstSession *BetaSession
	var err error

	if srcDevSpec == "" {
		srcSession = alpha.ActiveSession()
	} else {
		dev := strings.TrimPrefix(srcDevSpec, "$")
		srcSession, err = alpha.ResolveDevice(dev)
		if err != nil {
			return fmt.Errorf("source device: %w", err)
		}
	}

	if dstDevSpec == "" {
		dstSession = alpha.ActiveSession()
	} else {
		dev := strings.TrimPrefix(dstDevSpec, "$")
		dstSession, err = alpha.ResolveDevice(dev)
		if err != nil {
			return fmt.Errorf("destination device: %w", err)
		}
	}

	srcPath = normalizePath(srcPath)
	dstPath = normalizePath(dstPath)

	srcIsAlpha := srcSession == nil
	dstIsAlpha := dstSession == nil

	if srcIsAlpha && dstIsAlpha {
		return fmt.Errorf("local-to-local transfer not supported via @cp/@move; use OS commands")
	}

	switch {
	case srcIsAlpha && !dstIsAlpha:
		return pushFile(dstSession, srcPath, dstPath, move)
	case !srcIsAlpha && dstIsAlpha:
		return pullFile(srcSession, srcPath, dstPath, move)
	default:
		return relayTransfer(srcSession, dstSession, srcPath, dstPath, move)
	}
}

func pushFile(dst *BetaSession, localPath, remotePath string, move bool) error {
	info, err := os.Stat(localPath)
	if err != nil {
		return fmt.Errorf("local file: %w", err)
	}
	if info.IsDir() {
		return fmt.Errorf("local path is a directory; file transfers do not support directories")
	}

	f, err := os.Open(localPath)
	if err != nil {
		return fmt.Errorf("open local file: %w", err)
	}
	defer f.Close()

	remotePath = normalizeRemotePath(remotePath)

	if err := dst.Send(protocol.NewMessage(protocol.MsgFilePushReq, &protocol.FilePushReqPayload{
		Path: remotePath,
		Size: info.Size(),
	})); err != nil {
		return fmt.Errorf("send push req: %w", err)
	}

	msg, err := dst.Read()
	if err != nil {
		return fmt.Errorf("read push ack: %w", err)
	}
	if msg.Type == protocol.MsgError {
		var e protocol.ErrorPayload
		json.Unmarshal(msg.Payload, &e)
		return fmt.Errorf("beta error: %s", e.Message)
	}

	totalBytes := info.Size()
	var transferred int64
	buf := make([]byte, 65536)
	startTime := time.Now()
	lastUpdate := startTime

	for {
		n, err := f.Read(buf)
		if n > 0 {
			if err := dst.Send(protocol.NewMessage(protocol.MsgFilePushData, &protocol.FilePushDataPayload{
				Data: buf[:n],
			})); err != nil {
				return fmt.Errorf("send data: %w", err)
			}
			transferred += int64(n)

			if time.Since(lastUpdate) > 100*time.Millisecond {
				printProgress(transferred, totalBytes, startTime)
				lastUpdate = time.Now()
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("read local file: %w", err)
		}
	}

	if err := dst.Send(protocol.NewMessage(protocol.MsgFilePushEnd, nil)); err != nil {
		return fmt.Errorf("send push end: %w", err)
	}

	msg, err = dst.Read()
	if err != nil {
		return fmt.Errorf("read push done: %w", err)
	}
	if msg.Type == protocol.MsgError {
		var e protocol.ErrorPayload
		json.Unmarshal(msg.Payload, &e)
		return fmt.Errorf("beta error: %s", e.Message)
	}

	printProgress(transferred, totalBytes, startTime)
	fmt.Println()
	fmt.Printf("  Transfer complete: %s -> %s (%s)\n", localPath, remotePath, formatBytes(totalBytes))

	if move {
		os.Remove(localPath)
	}

	return nil
}

func pullFile(src *BetaSession, remotePath, localPath string, move bool) error {
	remotePath = normalizeRemotePath(remotePath)

	if err := src.Send(protocol.NewMessage(protocol.MsgFilePullReq, &protocol.FilePullReqPayload{
		Path: remotePath,
	})); err != nil {
		return fmt.Errorf("send pull req: %w", err)
	}

	msg, err := src.Read()
	if err != nil {
		return fmt.Errorf("read pull info: %w", err)
	}

	var totalBytes int64
	if msg.Type == protocol.MsgFilePullInfo {
		var info protocol.FilePullInfoPayload
		json.Unmarshal(msg.Payload, &info)
		totalBytes = info.Size
	} else if msg.Type == protocol.MsgError {
		var e protocol.ErrorPayload
		json.Unmarshal(msg.Payload, &e)
		return fmt.Errorf("beta: %s", e.Message)
	} else {
		return fmt.Errorf("unexpected response: %s", msg.Type)
	}

	dir := filepath.Dir(localPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create local directory: %w", err)
	}

	f, err := os.Create(localPath)
	if err != nil {
		return fmt.Errorf("create local file: %w", err)
	}
	defer f.Close()

	var transferred int64
	startTime := time.Now()
	lastUpdate := startTime

	if err := src.Send(protocol.NewMessage(protocol.MsgFilePullData, nil)); err != nil {
		return fmt.Errorf("request data: %w", err)
	}

	for {
		msg, err := src.Read()
		if err != nil {
			return fmt.Errorf("read data: %w", err)
		}

		switch msg.Type {
		case protocol.MsgFilePullData:
			var p protocol.FilePullDataPayload
			json.Unmarshal(msg.Payload, &p)
			if len(p.Data) > 0 {
				n, err := f.Write(p.Data)
				if err != nil {
					return fmt.Errorf("write local file: %w", err)
				}
				transferred += int64(n)

				if time.Since(lastUpdate) > 100*time.Millisecond {
					printProgress(transferred, totalBytes, startTime)
					lastUpdate = time.Now()
				}

				if err := src.Send(protocol.NewMessage(protocol.MsgFilePullData, nil)); err != nil {
					return fmt.Errorf("request next: %w", err)
				}
			}
		case protocol.MsgFilePullEnd:
			if err := src.Send(protocol.NewMessage(protocol.MsgFilePullOK, &protocol.FilePullOKPayload{Path: remotePath})); err != nil {
				return err
			}
			printProgress(transferred, totalBytes, startTime)
			fmt.Println()
			fmt.Printf("  Transfer complete: %s <- %s (%s)\n", localPath, remotePath, formatBytes(totalBytes))

			if move {
				src.Send(protocol.NewMessage(protocol.MsgExec, &protocol.ExecPayload{
					Cmd: fmt.Sprintf("rm %s", quotePath(remotePath)),
				}))
				for {
					m, _ := src.Read()
					if m != nil && m.Type == protocol.MsgExecDone {
						break
					}
				}
			}
			return nil
		case protocol.MsgError:
			var e protocol.ErrorPayload
			json.Unmarshal(msg.Payload, &e)
			return fmt.Errorf("beta: %s", e.Message)
		}
	}
}

func relayTransfer(src, dst *BetaSession, srcPath, dstPath string, move bool) error {
	srcPath = normalizeRemotePath(srcPath)
	dstPath = normalizeRemotePath(dstPath)

	tmpFile, err := os.CreateTemp("", "zhh-relay-*")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	tmpFile.Close()
	defer os.Remove(tmpPath)

	if err := pullFile(src, srcPath, tmpPath, false); err != nil {
		return fmt.Errorf("pull from source: %w", err)
	}

	if err := pushFile(dst, tmpPath, dstPath, false); err != nil {
		return fmt.Errorf("push to destination: %w", err)
	}

	if move {
		src.Send(protocol.NewMessage(protocol.MsgExec, &protocol.ExecPayload{
			Cmd: fmt.Sprintf("rm %s", quotePath(srcPath)),
		}))
		for {
			m, _ := src.Read()
			if m != nil && m.Type == protocol.MsgExecDone {
				break
			}
		}
	}

	return nil
}

func normalizePath(p string) string {
	p = filepath.FromSlash(p)
	return filepath.Clean(p)
}

func normalizeRemotePath(p string) string {
	p = filepath.ToSlash(p)
	return p
}

func quotePath(p string) string {
	if strings.Contains(p, " ") {
		return "'" + p + "'"
	}
	return p
}

func printProgress(transferred, total int64, start time.Time) {
	elapsed := time.Since(start).Seconds()
	var speed float64
	if elapsed > 0 {
		speed = float64(transferred) / elapsed
	}

	pct := float64(0)
	if total > 0 {
		pct = float64(transferred) * 100 / float64(total)
	}

	barWidth := 40
	filled := int(pct * float64(barWidth) / 100)
	bar := strings.Repeat("=", filled)
	if filled < barWidth {
		bar += ">"
		bar += strings.Repeat(" ", barWidth-filled-1)
	}

	fmt.Printf("\r  [%s] %5.1f%%  %s / %s  %s/s",
		bar, pct, formatBytes(transferred), formatBytes(total), formatBytes(int64(speed)))
}

func formatBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}
