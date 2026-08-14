package spinner

import (
	"bytes"
	"testing"
	"time"
)

// A non-terminal writer must be left untouched: Start is a no-op and Stop is
// safe even when nothing started.
func TestSpinnerNoOpWhenNotTerminal(t *testing.T) {
	var buf bytes.Buffer
	s := NewWriter(&buf, "working")
	s.Start()
	time.Sleep(10 * time.Millisecond)
	s.Stop()
	if buf.Len() != 0 {
		t.Fatalf("spinner wrote %q to a non-terminal writer", buf.String())
	}
}

// The progress list must leave a non-terminal writer untouched too.
func TestProgressListNoOpWhenNotTerminal(t *testing.T) {
	var buf bytes.Buffer
	p := NewProgressList(&buf, []string{"one", "two"})
	p.Start()
	p.Advance()
	p.Finish()
	if buf.Len() != 0 {
		t.Fatalf("progress list wrote %q to a non-terminal writer", buf.String())
	}
}

// An empty list must not panic through the full lifecycle.
func TestProgressListEmpty(t *testing.T) {
	p := NewProgressList(&bytes.Buffer{}, nil)
	p.Start()
	p.Advance()
	p.Finish()
}
