// Command test-renderer animates a spinner while forwarding test-result lines
// from stdin to stdout. test-runner.sh pipes its streamed results through this
// command so a terminal shows a spinner between test lines without the spinner
// and the results colliding on the same line.
//
// Usage: test-renderer [message]
package main

import (
	"bufio"
	"fmt"
	"os"

	"github.com/nisrulz/prlogue/internal/spinner"
)

func main() {
	msg := "Running tests"
	if len(os.Args) > 1 {
		msg = os.Args[1]
	}

	s := spinner.New(msg)
	s.Start()
	sc := bufio.NewScanner(os.Stdin)
	for sc.Scan() {
		s.Stop()
		fmt.Println(sc.Text())
		s.Start()
	}
	if err := sc.Err(); err != nil {
		fmt.Fprintln(os.Stderr, "test-renderer:", err)
		os.Exit(1)
	}
	s.Stop()
}
