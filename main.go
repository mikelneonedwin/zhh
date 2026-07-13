package main

import (
	"fmt"
	"os"
	"strings"

	"zhh/alpha"
	"zhh/beta"
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

	default:
		fmt.Fprintf(os.Stderr, "Usage: zhh [alpha|a] [target] [- command]\n")
		fmt.Fprintf(os.Stderr, "       zhh [beta|b]               (default)\n")
		os.Exit(1)
	}
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
