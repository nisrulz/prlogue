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
