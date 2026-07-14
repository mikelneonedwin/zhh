package beta

import (
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"

	"zhh/discovery"
	"zhh/protocol"
)

func Run(port int) {
	hostname, _ := os.Hostname()
	ips := getLocalIPs()
	octet := 0
	for _, ip := range ips {
		if strings.Count(ip, ".") == 3 {
			parts := strings.Split(ip, ".")
			octet, _ = strconv.Atoi(parts[len(parts)-1])
			break
		}
	}

	mdnsServer, err := discovery.Register(port, hostname, octet, runtime.GOOS)
	if err != nil {
		log.Printf("Warning: mDNS registration failed: %v", err)
	} else {
		defer mdnsServer.Shutdown()
	}

	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		log.Fatalf("Failed to listen on port %d: %v", port, err)
	}
	defer listener.Close()

	fmt.Printf("\n  zhh beta ready\n")
	fmt.Printf("  Hostname: %s\n", hostname)
	fmt.Printf("  OS:       %s / %s\n", runtime.GOOS, runtime.GOARCH)
	fmt.Printf("  Port:     %d\n", port)
	fmt.Printf("  IPs:\n")
	for _, ip := range ips {
		fmt.Printf("    %s\n", ip)
	}
	fmt.Printf("  Octet:    %d  (use on alpha: zhh a %d)\n", octet, octet)
	fmt.Printf("  ---\n\n")

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Printf("Accept error: %v", err)
			continue
		}
		go handleConnection(conn, hostname, octet)
	}
}

func getLocalIPs() []string {
	var ips []string
	interfaces, err := net.Interfaces()
	if err != nil {
		return ips
	}
	for _, iface := range interfaces {
		if iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			ipnet, ok := addr.(*net.IPNet)
			if !ok {
				continue
			}
			if ipnet.IP.To4() != nil {
				ips = append(ips, ipnet.IP.String())
			}
		}
	}
	return ips
}

func handleConnection(conn net.Conn, hostname string, octet int) {
	defer conn.Close()
	log.Printf("Connection from %s", conn.RemoteAddr())

	session := NewSession()

	ident := protocol.NewMessage(protocol.MsgIdentify, &protocol.IdentifyPayload{
		Hostname: hostname,
		OS:       runtime.GOOS,
		Version:  getOSVersion(),
		Shells:   session.Shells,
		Octet:    octet,
		IP:       conn.LocalAddr().String(),
	})
	if err := protocol.WriteMessage(conn, ident); err != nil {
		log.Printf("Write identify: %v", err)
		return
	}

	for {
		msg, err := protocol.ReadMessage(conn)
		if err != nil {
			if err != io.EOF {
				log.Printf("Read: %v", err)
			}
			return
		}

		if err := handleMessage(conn, session, msg); err != nil {
			log.Printf("Handle %s: %v", msg.Type, err)
			errMsg := protocol.NewMessage(protocol.MsgError, &protocol.ErrorPayload{Message: err.Error()})
			protocol.WriteMessage(conn, errMsg)
		}
	}
}

func handleMessage(conn net.Conn, session *Session, msg *protocol.Message) error {
	switch msg.Type {
	case protocol.MsgExec:
		return handleExec(conn, session, msg)
	case protocol.MsgCD:
		return handleCD(conn, session, msg)
	case protocol.MsgShellSwitch:
		return handleShellSwitch(conn, session, msg)
	case protocol.MsgShellList:
		return handleShellList(conn, session, msg)
	case protocol.MsgFilePushReq:
		return handleFilePushReq(conn, msg)
	case protocol.MsgFilePushData:
		return handleFilePushData(conn, msg)
	case protocol.MsgFilePushEnd:
		return handleFilePushEnd(conn)
	case protocol.MsgFilePullReq:
		return handleFilePullReq(conn, msg)
	case protocol.MsgFilePullData:
		return handleFilePullData(conn, msg)
	case protocol.MsgFilePullEnd:
		return handleFilePullEnd(conn)
	case protocol.MsgWhoami:
		return handleWhoami(conn, session)
	case protocol.MsgHeartbeat:
		return nil
	default:
		return fmt.Errorf("unknown message type: %s", msg.Type)
	}
}

func handleExec(conn net.Conn, session *Session, msg *protocol.Message) error {
	var payload protocol.ExecPayload
	if msg.Payload != nil {
		if err := protocol.DecodePayload(msg.Payload, &payload); err != nil {
			return err
		}
	}

	code, cwd, err := session.HandleExec(
		payload.Cmd,
		payload.Stdin,
		func(data []byte) {
			protocol.WriteMessage(conn, protocol.NewMessage(protocol.MsgExecStdout, &protocol.ExecOutputPayload{Data: data}))
		},
		func(data []byte) {
			protocol.WriteMessage(conn, protocol.NewMessage(protocol.MsgExecStderr, &protocol.ExecOutputPayload{Data: data}))
		},
	)
	if err != nil {
		return err
	}

	return protocol.WriteMessage(conn, protocol.NewMessage(protocol.MsgExecDone, &protocol.ExecDonePayload{
		Code: code,
		Cwd:  cwd,
	}))
}

func handleCD(conn net.Conn, session *Session, msg *protocol.Message) error {
	var payload protocol.CDPayload
	if msg.Payload != nil {
		if err := protocol.DecodePayload(msg.Payload, &payload); err != nil {
			return err
		}
	}
	newCwd, err := session.HandleCD(payload.Dir)
	if err != nil {
		return protocol.WriteMessage(conn, protocol.NewMessage(protocol.MsgError, &protocol.ErrorPayload{Message: err.Error()}))
	}
	return protocol.WriteMessage(conn, protocol.NewMessage(protocol.MsgExecDone, &protocol.ExecDonePayload{
		Code: 0,
		Cwd:  newCwd,
	}))
}

func handleShellSwitch(conn net.Conn, session *Session, msg *protocol.Message) error {
	var payload protocol.ShellSwitchPayload
	if msg.Payload != nil {
		if err := protocol.DecodePayload(msg.Payload, &payload); err != nil {
			return err
		}
	}
	if !session.SetShell(payload.Shell) {
		return protocol.WriteMessage(conn, protocol.NewMessage(protocol.MsgError, &protocol.ErrorPayload{
			Message: fmt.Sprintf("shell %q not available", payload.Shell),
		}))
	}
	return protocol.WriteMessage(conn, protocol.NewMessage(protocol.MsgExecDone, &protocol.ExecDonePayload{
		Code: 0,
		Cwd:  session.GetCwd(),
	}))
}

func handleShellList(conn net.Conn, session *Session, msg *protocol.Message) error {
	return protocol.WriteMessage(conn, protocol.NewMessage(protocol.MsgShellList, &protocol.ShellListPayload{
		Shells: session.Shells,
	}))
}

// File transfer state
var (
	pushFile   *os.File
	pullFile   *os.File
	pushFileMu sync.Mutex
	pullFileMu sync.Mutex
)

func handleFilePushReq(conn net.Conn, msg *protocol.Message) error {
	var payload protocol.FilePushReqPayload
	if msg.Payload != nil {
		if err := protocol.DecodePayload(msg.Payload, &payload); err != nil {
			return err
		}
	}

	dir := filepath.Dir(payload.Path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	f, err := os.Create(payload.Path)
	if err != nil {
		return err
	}

	pushFileMu.Lock()
	if pushFile != nil {
		pushFile.Close()
	}
	pushFile = f
	pushFileMu.Unlock()

	return protocol.WriteMessage(conn, protocol.NewMessage(protocol.MsgFilePushOK, nil))
}

func handleFilePushData(conn net.Conn, msg *protocol.Message) error {
	var payload protocol.FilePushDataPayload
	if msg.Payload != nil {
		if err := protocol.DecodePayload(msg.Payload, &payload); err != nil {
			return err
		}
	}

	pushFileMu.Lock()
	defer pushFileMu.Unlock()
	if pushFile == nil {
		return fmt.Errorf("no active file push")
	}

	_, err := pushFile.Write(payload.Data)
	return err
}

func handleFilePushEnd(conn net.Conn) error {
	pushFileMu.Lock()
	if pushFile != nil {
		pushFile.Close()
		pushFile = nil
	}
	pushFileMu.Unlock()
	return protocol.WriteMessage(conn, protocol.NewMessage(protocol.MsgFilePushOK, &protocol.FilePushOKPayload{Path: ""}))
}

func handleFilePullReq(conn net.Conn, msg *protocol.Message) error {
	var payload protocol.FilePullReqPayload
	if msg.Payload != nil {
		if err := protocol.DecodePayload(msg.Payload, &payload); err != nil {
			return err
		}
	}

	info, err := os.Stat(payload.Path)
	if err != nil {
		return protocol.WriteMessage(conn, protocol.NewMessage(protocol.MsgError, &protocol.ErrorPayload{Message: err.Error()}))
	}

	if info.IsDir() {
		return protocol.WriteMessage(conn, protocol.NewMessage(protocol.MsgError, &protocol.ErrorPayload{
			Message: fmt.Sprintf("%s is a directory", payload.Path),
		}))
	}

	f, err := os.Open(payload.Path)
	if err != nil {
		return protocol.WriteMessage(conn, protocol.NewMessage(protocol.MsgError, &protocol.ErrorPayload{Message: err.Error()}))
	}

	pushFileMu.Lock()
	if pullFile != nil {
		pullFile.Close()
	}
	pullFile = f
	pushFileMu.Unlock()

	return protocol.WriteMessage(conn, protocol.NewMessage(protocol.MsgFilePullInfo, &protocol.FilePullInfoPayload{
		Size: info.Size(),
	}))
}

func handleFilePullData(conn net.Conn, msg *protocol.Message) error {
	var payload protocol.FilePullDataPayload
	if msg.Payload != nil {
		if err := protocol.DecodePayload(msg.Payload, &payload); err != nil {
			return err
		}
	}

	pushFileMu.Lock()
	defer pushFileMu.Unlock()
	if pullFile == nil {
		return fmt.Errorf("no active file pull")
	}

	buf := make([]byte, 65536)
	n, err := pullFile.Read(buf)
	if n > 0 {
		return protocol.WriteMessage(conn, protocol.NewMessage(protocol.MsgFilePullData, &protocol.FilePullDataPayload{
			Data: buf[:n],
		}))
	}
	if err == io.EOF {
		pullFile.Close()
		pullFile = nil
		return protocol.WriteMessage(conn, protocol.NewMessage(protocol.MsgFilePullEnd, nil))
	}
	return err
}

func handleFilePullEnd(conn net.Conn) error {
	pushFileMu.Lock()
	if pullFile != nil {
		pullFile.Close()
		pullFile = nil
	}
	pushFileMu.Unlock()
	return protocol.WriteMessage(conn, protocol.NewMessage(protocol.MsgFilePullOK, &protocol.FilePullOKPayload{Path: ""}))
}

func handleWhoami(conn net.Conn, session *Session) error {
	hostname, _ := os.Hostname()
	user := os.Getenv("USER")
	if user == "" {
		user = os.Getenv("USERNAME")
	}

	storage := getStorageInfo()
	battery := getBatteryInfo()

	return protocol.WriteMessage(conn, protocol.NewMessage(protocol.MsgWhoamiResp, &protocol.WhoamiRespPayload{
		User:     user,
		Hostname: hostname,
		IP:       conn.LocalAddr().String(),
		OS:       runtime.GOOS,
		Version:  getOSVersion(),
		Storage:  storage,
		Battery:  battery,
		Shell:    session.GetShell(),
		Cwd:      session.GetCwd(),
	}))
}

func getOSVersion() string {
	if runtime.GOOS == "windows" {
		return runtime.GOOS + " (unknown build)"
	}
	data, err := os.ReadFile("/proc/version")
	if err != nil {
		return runtime.GOOS
	}
	parts := strings.SplitN(string(data), " ", 3)
	if len(parts) >= 2 {
		return strings.TrimSpace(parts[0] + " " + parts[1])
	}
	return runtime.GOOS
}

func getStorageInfo() string {
	return "N/A (cross-platform support pending)"
}

func getBatteryInfo() string {
	return "N/A"
}
