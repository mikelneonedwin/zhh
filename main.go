package main

import (
	"fmt"
	"log"
	"net"
	"os"
	"strings"
	"time"

	"zhh/alpha"
	"zhh/beta"
	"zhh/protocol"
)

func main() {
	port := 9999
	args := os.Args[1:]

	if len(args) == 0 {
		beta.Run(port)
		return
	}

	switch args[0] {
	case "alpha", "a":
		target, command := parseAlphaArgs(args[1:])
		alpha.Run(target, command)

	case "beta", "b":
		beta.Run(port)

	case "twin", "t":
		command, _ := parseAlphaArgs(args[1:])
		runTwin(port, command)

	default:
		fmt.Fprintf(os.Stderr, "Usage: zhh [alpha|a] [target] [- command]\n")
		fmt.Fprintf(os.Stderr, "       zhh [beta|b]               (default)\n")
		fmt.Fprintf(os.Stderr, "       zhh [twin|t]               (local test)\n")
		os.Exit(1)
	}
}

func runTwin(port int, command string) {
	go beta.Run(port)
	time.Sleep(500 * time.Millisecond)

	a := alpha.NewAlpha()

	conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 5*time.Second)
	if err != nil {
		log.Fatalf("twin connect: %v", err)
	}

	log.Printf("Connected to local beta on 127.0.0.1:%d", port)

	identMsg, err := protocol.ReadMessage(conn)
	if err != nil {
		log.Fatalf("twin identify: %v", err)
	}

	a.AddSession(conn, identMsg, 1)

	if command != "" {
		alpha.RunSingleCommand(a, command)
		return
	}

	alpha.RunInteractive(a)
}

func parseAlphaArgs(args []string) (target, command string) {
	sepIdx := -1
	for i, arg := range args {
		if arg == "-" {
			sepIdx = i
			break
		}
	}

	if sepIdx >= 0 {
		target = strings.Join(args[:sepIdx], " ")
		command = strings.Join(args[sepIdx+1:], " ")
	} else if len(args) > 0 {
		target = strings.Join(args, " ")
	}

	return
}
