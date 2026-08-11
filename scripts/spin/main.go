// Command spin runs a command while the braille spinner animates on stderr.
// It lets the Makefile reuse the spinner used by the CLI and the e2e suite.
//
// Usage: spin <message> -- <command> [args...]
package main

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/nisrulz/prlogue/internal/spinner"
)

func main() {
	if len(os.Args) < 4 || os.Args[2] != "--" {
		fmt.Fprintln(os.Stderr, "usage: spin <message> -- <command> [args...]")
		os.Exit(2)
	}
	msg := os.Args[1]
	args := os.Args[3:]

	s := spinner.New(msg)
	s.Start()
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	s.Stop()
	if err != nil {
		os.Exit(1)
	}
}
