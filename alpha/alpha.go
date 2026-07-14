package alpha

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"zhh/discovery"
	"zhh/protocol"
)

type BetaSession struct {
	ID       int
	Octet    int
	Conn     net.Conn
	Hostname string
	OS       string
	Shell    string
	Cwd      string
	Shells   []string
	mu       sync.Mutex
}

func (s *BetaSession) Send(msg *protocol.Message) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return protocol.WriteMessage(s.Conn, msg)
}

func (s *BetaSession) SendAndRecv(msgType string, payload interface{}) (*protocol.Message, error) {
	s.mu.Lock()
	err := protocol.WriteMessage(s.Conn, protocol.NewMessage(msgType, payload))
	s.mu.Unlock()
	if err != nil {
		return nil, err
	}
	return protocol.ReadMessage(s.Conn)
}

func (s *BetaSession) Read() (*protocol.Message, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return protocol.ReadMessage(s.Conn)
}

func (s *BetaSession) ReadWithTimeout(timeout time.Duration) (*protocol.Message, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Conn.SetReadDeadline(time.Now().Add(timeout))
	defer s.Conn.SetReadDeadline(time.Time{})
	return protocol.ReadMessage(s.Conn)
}

var (
	deviceColors = []string{
		"\033[31m", "\033[32m", "\033[33m", "\033[34m",
		"\033[35m", "\033[36m", "\033[91m", "\033[92m",
		"\033[93m", "\033[94m", "\033[95m", "\033[96m",
	}
	resetCode = "\033[0m"
)

func getDeviceColor(spec string) string {
	if spec == "" || spec == "active" {
		return ""
	}
	h := 0
	for _, c := range spec {
		h = h*31 + int(c)
	}
	if h < 0 {
		h = -h
	}
	return deviceColors[h%len(deviceColors)]
}

type Alpha struct {
	sessions  map[int]*BetaSession
	octetSess map[int]*BetaSession
	nextID    int
	activeID  int
	mu        sync.Mutex
	localWD   string
}

func NewAlpha() *Alpha {
	wd, _ := os.Getwd()
	return &Alpha{
		sessions:  make(map[int]*BetaSession),
		octetSess: make(map[int]*BetaSession),
		nextID:    2,
		activeID:  0,
		localWD:   wd,
	}
}

func (a *Alpha) AddSession(conn net.Conn, identMsg *protocol.Message, octet int) *BetaSession {
	var ident protocol.IdentifyPayload
	if identMsg != nil && identMsg.Payload != nil {
		json.Unmarshal(identMsg.Payload, &ident)
	}

	session := &BetaSession{
		ID:       a.nextID,
		Octet:    ident.Octet,
		Conn:     conn,
		Hostname: ident.Hostname,
		OS:       ident.OS,
		Shell:    "?",
		Cwd:      "/",
		Shells:   ident.Shells,
	}

	if len(ident.Shells) > 0 {
		session.Shell = ident.Shells[0]
	}

	a.mu.Lock()
	a.sessions[a.nextID] = session
	if ident.Octet > 0 {
		a.octetSess[ident.Octet] = session
	}
	if a.activeID == 0 {
		a.activeID = a.nextID
	}
	a.nextID++
	a.mu.Unlock()

	return session
}

func (a *Alpha) GetSession(id int) *BetaSession {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.sessions[id]
}

func (a *Alpha) ActiveSession() *BetaSession {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.sessions[a.activeID]
}

func (a *Alpha) SetActive(id int) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	if _, ok := a.sessions[id]; ok {
		a.activeID = id
		return true
	}
	return false
}

func (a *Alpha) SessionList() []*BetaSession {
	a.mu.Lock()
	defer a.mu.Unlock()
	var list []*BetaSession
	for _, s := range a.sessions {
		list = append(list, s)
	}
	return list
}

func (a *Alpha) RemoveSession(id int) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if s, ok := a.sessions[id]; ok {
		s.Conn.Close()
		delete(a.sessions, id)
		if s.Octet > 0 {
			delete(a.octetSess, s.Octet)
		}
		if a.activeID == id {
			// Switch to another session
			for k := range a.sessions {
				a.activeID = k
				return
			}
			a.activeID = 0
		}
	}
}

func (a *Alpha) ResolveDevice(spec string) (*BetaSession, error) {
	spec = strings.TrimSpace(spec)
	if spec == "" {
		return nil, fmt.Errorf("empty device spec")
	}

	switch spec {
	case "alpha", "a", "1":
		return nil, nil
	case "beta", "b":
		a.mu.Lock()
		defer a.mu.Unlock()
		for id, s := range a.sessions {
			if id == 2 {
				return s, nil
			}
		}
		// Fallback: return first session
		for _, s := range a.sessions {
			return s, nil
		}
		return nil, fmt.Errorf("no betas connected")
	}

	// Try numeric
	if num, err := strconv.Atoi(spec); err == nil {
		if num == 1 {
			return nil, nil
		}
		a.mu.Lock()
		if s, ok := a.sessions[num]; ok {
			a.mu.Unlock()
			return s, nil
		}
		a.mu.Unlock()
		// Try octet
		a.mu.Lock()
		for _, s := range a.sessions {
			if s.Octet == num {
				a.mu.Unlock()
				return s, nil
			}
		}
		a.mu.Unlock()
		return nil, fmt.Errorf("no device with ID or octet %d", num)
	}

	// Dot-prefixed octet: .42
	if strings.HasPrefix(spec, ".") {
		if num, err := strconv.Atoi(spec[1:]); err == nil {
			a.mu.Lock()
			for _, s := range a.sessions {
				if s.Octet == num {
					a.mu.Unlock()
					return s, nil
				}
			}
			a.mu.Unlock()
			return nil, fmt.Errorf("no beta with octet %d", num)
		}
		return nil, fmt.Errorf("invalid octet: %s", spec)
	}

	return nil, fmt.Errorf("unknown device: %s", spec)
}

func Run(target, command string) {
	alpha := NewAlpha()

	// Discover betas
	var targetOctet int
	if target != "" {
		targetOctet, _ = strconv.Atoi(target)
	}

	log.Printf("Discovering betas on network...")
	betas, err := discovery.Discover(nil, targetOctet)
	if err != nil {
		log.Fatalf("Discovery failed: %v", err)
	}
	if len(betas) == 0 {
		if targetOctet > 0 {
			log.Fatalf("No beta found with octet %d", targetOctet)
		}
		log.Printf("No betas discovered via mDNS. Waiting 3 more seconds...")
		betas, err = discovery.Discover(nil, targetOctet)
		if err != nil {
			log.Fatalf("Discovery failed: %v", err)
		}
		if len(betas) == 0 {
			log.Fatalf("No betas found on network")
		}
	}

	for _, beta := range betas {
		log.Printf("Found beta: %s at %s:%d (octet %d)", beta.Hostname, beta.IP, beta.Port, beta.Octet)
		conn, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", beta.IP, beta.Port), 5*time.Second)
		if err != nil {
			log.Printf("  Connect failed: %v", err)
			continue
		}
		// Read identify message
		identMsg, err := protocol.ReadMessage(conn)
		if err != nil {
			log.Printf("  Identify read failed: %v", err)
			conn.Close()
			continue
		}
		var ident protocol.IdentifyPayload
		if identMsg.Payload != nil {
			json.Unmarshal(identMsg.Payload, &ident)
		}
		alpha.AddSession(conn, identMsg, beta.Octet)
		log.Printf("  Connected: %s (%s)", ident.Hostname, ident.OS)
	}

	if alpha.activeID == 0 {
		log.Fatalf("Failed to connect to any beta")
	}

	if command != "" {
		runSingleCommand(alpha, command)
		return
	}

	runInteractive(alpha)
}

func runSingleCommand(alpha *Alpha, command string) {
	session := alpha.ActiveSession()
	if session == nil {
		log.Fatalf("No active session")
	}

	stdoutDone := make(chan struct{})
	stderrDone := make(chan struct{})

	if err := session.Send(protocol.NewMessage(protocol.MsgExec, &protocol.ExecPayload{
		Cmd: command,
	})); err != nil {
		log.Fatalf("Send exec: %v", err)
	}

	go func() {
		for {
			msg, err := session.Read()
			if err != nil {
				break
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
				os.Exit(p.Code)
			case protocol.MsgError:
				var p protocol.ErrorPayload
				json.Unmarshal(msg.Payload, &p)
				fmt.Fprintf(os.Stderr, "Error: %s\n", p.Message)
				os.Exit(1)
			}
		}
		close(stdoutDone)
		close(stderrDone)
	}()

	<-stdoutDone
	<-stderrDone
}

func runInteractive(alpha *Alpha) {
	reader := bufio.NewReader(os.Stdin)
	fmt.Println()
	fmt.Println("  zhh alpha interactive mode")
	fmt.Println("  Type @help for available commands")
	fmt.Println()

	for {
		session := alpha.ActiveSession()
		if session == nil {
			fmt.Println("No active session. Exiting.")
			return
		}

		prompt := fmt.Sprintf("%s@%s:%s$ ", session.Hostname, session.Shell, session.Cwd)
		fmt.Print(prompt)

		line, err := reader.ReadString('\n')
		if err != nil {
			if err != io.EOF {
				fmt.Fprintf(os.Stderr, "Read error: %v\n", err)
			}
			break
		}

		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		if strings.HasPrefix(line, "@") {
			if err := handleMetaCommand(alpha, line); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			}
			continue
		}

		if strings.HasPrefix(line, "#") {
			if err := handleShellCommand(session, line); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			}
			continue
		}

		if err := alpha.executePipeline(line); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		}
	}
}

func handleMetaCommand(alpha *Alpha, line string) error {
	parts := strings.Fields(line)
	cmd := parts[0]

	switch cmd {
	case "@switch":
		return handleSwitch(alpha)
	case "@whoami":
		return handleWhoami(alpha)
	case "@cp", "@copy":
		if len(parts) < 3 {
			return fmt.Errorf("usage: %s [src_dev] <src_path> [dst_dev] <dst_path>", cmd)
		}
		srcDev, srcPath, dstDev, dstPath, err := parseTransferArgs(parts[1:])
		if err != nil {
			return err
		}
		return handleFileTransfer(alpha, srcDev, srcPath, dstDev, dstPath, false)
	case "@mv", "@move":
		if len(parts) < 3 {
			return fmt.Errorf("usage: %s [src_dev] <src_path> [dst_dev] <dst_path>", cmd)
		}
		srcDev, srcPath, dstDev, dstPath, err := parseTransferArgs(parts[1:])
		if err != nil {
			return err
		}
		return handleFileTransfer(alpha, srcDev, srcPath, dstDev, dstPath, true)
	case "@help":
		return handleHelp()
	case "@exit", "@quit":
		os.Exit(0)
	default:
		return fmt.Errorf("unknown command: %s (try @help)", cmd)
	}
	return nil
}

func handleSwitch(alpha *Alpha) error {
	sessions := alpha.SessionList()
	if len(sessions) == 0 {
		fmt.Println("No connected betas.")
		return nil
	}

	active := alpha.ActiveSession()

	fmt.Println()
	fmt.Println("  Connected devices:")
	fmt.Printf("  [1]  alpha (local)\n")
	for _, s := range sessions {
		marker := " "
		if active != nil && s.ID == active.ID {
			marker = ">"
		}
		fmt.Printf("  %s[%d]  %s@%s  (octet %d, %s)\n",
			marker, s.ID, s.Hostname, s.Shell, s.Octet, s.OS)
	}
	fmt.Printf("  Active: %d\n", active.ID)
	fmt.Print("  Switch to device number (or Enter to cancel): ")

	reader := bufio.NewReader(os.Stdin)
	line, _ := reader.ReadString('\n')
	line = strings.TrimSpace(line)
	if line == "" {
		return nil
	}

	num, err := strconv.Atoi(line)
	if err != nil {
		return fmt.Errorf("invalid number: %s", line)
	}

	if num == 1 {
		fmt.Println("Cannot switch to alpha (local). Use @cp/@move to transfer files.")
		return nil
	}

	if alpha.SetActive(num) {
		fmt.Printf("Switched to beta %d\n", num)
	} else {
		return fmt.Errorf("no beta with ID %d", num)
	}
	return nil
}

func handleWhoami(alpha *Alpha) error {
	session := alpha.ActiveSession()
	if session == nil {
		return fmt.Errorf("no active session")
	}

	if err := session.Send(protocol.NewMessage(protocol.MsgWhoami, nil)); err != nil {
		return fmt.Errorf("send whoami: %w", err)
	}

	for {
		msg, err := session.Read()
		if err != nil {
			return fmt.Errorf("read: %w", err)
		}
		switch msg.Type {
		case protocol.MsgWhoamiResp:
			var p protocol.WhoamiRespPayload
			if msg.Payload != nil {
				json.Unmarshal(msg.Payload, &p)
			}
			fmt.Println()
			fmt.Printf("  User:       %s\n", p.User)
			fmt.Printf("  Hostname:   %s\n", p.Hostname)
			fmt.Printf("  IP:         %s\n", p.IP)
			fmt.Printf("  OS:         %s / %s\n", p.OS, p.Version)
			fmt.Printf("  Storage:    %s\n", p.Storage)
			fmt.Printf("  Battery:    %s\n", p.Battery)
			fmt.Printf("  Shell:      %s\n", p.Shell)
			fmt.Printf("  Directory:  %s\n", p.Cwd)
			fmt.Println()
			return nil
		case protocol.MsgError:
			var p protocol.ErrorPayload
			json.Unmarshal(msg.Payload, &p)
			return fmt.Errorf("beta error: %s", p.Message)
		}
	}
}

func handleShellCommand(session *BetaSession, line string) error {
	line = strings.TrimSpace(line)

	if line == "#" {
		if err := session.Send(protocol.NewMessage(protocol.MsgShellList, nil)); err != nil {
			return err
		}
		msg, err := session.Read()
		if err != nil {
			return err
		}
		if msg.Type == protocol.MsgShellList {
			var p protocol.ShellListPayload
			json.Unmarshal(msg.Payload, &p)
			fmt.Println("  Available shells:")
			for _, sh := range p.Shells {
				fmt.Printf("    #%s\n", sh)
			}
		}
		return nil
	}

	shellName := strings.TrimPrefix(line, "#")
	shellName = strings.TrimSpace(shellName)

	if err := session.Send(protocol.NewMessage(protocol.MsgShellSwitch, &protocol.ShellSwitchPayload{
		Shell: shellName,
	})); err != nil {
		return err
	}

	msg, err := session.Read()
	if err != nil {
		return err
	}

	switch msg.Type {
	case protocol.MsgExecDone:
		var p protocol.ExecDonePayload
		json.Unmarshal(msg.Payload, &p)
		if p.Code == 0 {
			session.mu.Lock()
			session.Shell = shellName
			session.mu.Unlock()
			fmt.Printf("Switched to shell: %s\n", shellName)
		}
	case protocol.MsgError:
		var p protocol.ErrorPayload
		json.Unmarshal(msg.Payload, &p)
		return fmt.Errorf("shell switch failed: %s", p.Message)
	}
	return nil
}

func handleHelp() error {
	fmt.Println()
	fmt.Println("  zhh commands:")
	fmt.Println("    @switch           List and switch between connected betas")
	fmt.Println("    @whoami           Show active beta system information")
	fmt.Println("    @cp <src> <dst>   Copy file between devices")
	fmt.Println("    @copy <src> <dst> Alias for @cp")
	fmt.Println("    @mv <src> <dst>   Move file between devices")
	fmt.Println("    @move <src> <dst> Alias for @mv")
	fmt.Println("    @help             Show this help")
	fmt.Println("    @exit / @quit     Exit")
	fmt.Println()
	fmt.Println("  Shell commands:")
	fmt.Println("    #                 List available shells on beta")
	fmt.Println("    #bash / #cmd      Switch beta's active shell")
	fmt.Println()
	fmt.Println("  Pipeline: use | to chain, $prefix for device-specific stage")
	fmt.Println("    $   alone runs stage on alpha (local)")
	fmt.Println("    $N  runs stage on beta with ID N")
	fmt.Println("    $.N runs stage on beta with octet N")
	fmt.Println("    Example: ipconfig | $2 grep 192 | clip")
	fmt.Println("    Example: ipconfig | $grep 2 \"192\" | $.21 clip")
	fmt.Println()
	fmt.Println("  File transfer: @cp [src_dev] <src_path> [dst_dev] <dst_path>")
	fmt.Println("    Device defaults to active beta if omitted")
	fmt.Println("    Example: @cp /local/file $2/remote/dir")
	fmt.Println("    Example: @cp $2 /src $3 /dst")
	fmt.Println("    Example: @cp /src /dst        (both on active beta)")
	fmt.Println()
	return nil
}
