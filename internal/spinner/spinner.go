// Package spinner provides a braille progress spinner for terminal output.
package spinner

import (
	"fmt"
	"io"
	"os"
	"time"
)

// frames is the braille spinner animation, in order.
var frames = []rune("⠋⠙⠹⠸⠼⠴⠦⠧⠇⠏")

// refresh is the delay between frames.
const refresh = 80 * time.Millisecond

// Spinner renders an animated braille spinner and a message to a writer while
// a long-running operation is in progress. It animates only when the writer is
// a terminal, so piped output stays clean.
type Spinner struct {
	w       io.Writer
	message string
	stop    chan struct{}
	done    chan struct{}
	running bool
}

// New returns a spinner that writes to os.Stderr with the given message.
func New(message string) *Spinner {
	return NewWriter(os.Stderr, message)
}

// NewWriter returns a spinner that writes to w with the given message.
func NewWriter(w io.Writer, message string) *Spinner {
	return &Spinner{w: w, message: message}
}

// Start begins animating. It is a no-op when w is not a terminal.
func (s *Spinner) Start() {
	if !animatable(s.w) {
		return
	}
	s.stop = make(chan struct{})
	s.done = make(chan struct{})
	s.running = true
	go s.run()
}

// Stop ends the animation and clears the line. It is a no-op when Start was a
// no-op or Stop was already called.
func (s *Spinner) Stop() {
	if !s.running {
		return
	}
	s.running = false
	close(s.stop)
	<-s.done
	fmt.Fprint(s.w, "\r\x1b[2K")
}

func (s *Spinner) run() {
	defer close(s.done)
	for i := 0; ; i++ {
		select {
		case <-s.stop:
			return
		case <-time.After(refresh):
		}
		frame := string(frames[i%len(frames)])
		if s.message != "" {
			fmt.Fprintf(s.w, "\r%s %s", frame, s.message)
		} else {
			fmt.Fprint(s.w, "\r"+frame)
		}
	}
}
