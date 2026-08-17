package spinner

import (
	"bytes"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"
)

// fakeCharDevice is a writer that looks like a terminal (char device) to the
// capability check while still capturing what is written.
type fakeCharDevice struct {
	bytes.Buffer
}

func (f *fakeCharDevice) Stat() (os.FileInfo, error) {
	return fakeFileInfo{mode: os.ModeCharDevice}, nil
}

type fakeFileInfo struct{ mode os.FileMode }

func (f fakeFileInfo) Name() string       { return "" }
func (f fakeFileInfo) Size() int64        { return 0 }
func (f fakeFileInfo) Mode() os.FileMode  { return f.mode }
func (f fakeFileInfo) ModTime() time.Time { return time.Time{} }
func (f fakeFileInfo) IsDir() bool        { return false }
func (f fakeFileInfo) Sys() any           { return nil }

func TestAnimatable(t *testing.T) {
	if !animatable(&fakeCharDevice{}) {
		t.Fatal("char device must animate")
	}
	if animatable(&bytes.Buffer{}) {
		t.Fatal("non-terminal must not animate")
	}
}

// A terminal must animate with carriage returns only: one tick line per
// completed item, in order, with line clears but no cursor-movement escapes.
func TestProgressListAnimatesWithoutCursorUp(t *testing.T) {
	dev := &fakeCharDevice{}
	p := NewProgressList(dev, []string{"one", "two"})
	p.Start()
	p.Advance()
	p.Advance()
	p.Finish()
	got := dev.String()

	cursorMove := regexp.MustCompile(`\x1b\[[0-9;]*[ABCDG]`)
	if cursorMove.MatchString(got) {
		t.Fatalf("progress list emitted cursor-movement escape: %q", got)
	}
	first := bytes.Index([]byte(got), []byte("✓ one\n"))
	second := bytes.Index([]byte(got), []byte("✓ two\n"))
	if first < 0 || second < 0 || first > second {
		t.Fatalf("tick lines missing or out of order: %q", got)
	}
	if strings.Count(got, "✓ ") != 2 {
		t.Fatalf("expected one tick per item, got %q", got)
	}
}

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

// A non-terminal writer gets a static list: no animation, just one tick line
// per completed item.
func TestProgressListStaticWhenNotTerminal(t *testing.T) {
	var buf bytes.Buffer
	p := NewProgressList(&buf, []string{"one", "two"})
	p.Start()
	p.Advance()
	p.Advance()
	p.Finish()
	if got := buf.String(); got != "✓ one\n✓ two\n" {
		t.Fatalf("progress list wrote %q", got)
	}
}

// An empty list must not panic through the full lifecycle.
func TestProgressListEmpty(t *testing.T) {
	p := NewProgressList(&bytes.Buffer{}, nil)
	p.Start()
	p.Advance()
	p.Finish()
}
