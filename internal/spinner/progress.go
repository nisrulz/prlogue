package spinner

import (
	"fmt"
	"io"
	"os"
	"sync"
	"time"
)

// ProgressList renders a fixed list of items with per-item state. Items that
// are done get a tick, the item being processed gets the braille spinner, and
// pending items stay blank. It renders only when the writer is a terminal, so
// piped output stays clean.
type ProgressList struct {
	w     io.Writer
	items []string

	mu      sync.Mutex
	active  int
	stop    chan struct{}
	done    chan struct{}
	anim    bool
	running bool
	drawn   bool
}

// NewProgressList returns a progress list that writes to w.
func NewProgressList(w io.Writer, items []string) *ProgressList {
	return &ProgressList{w: w, items: items}
}

// Start begins rendering the list. It is a no-op when w is not a terminal or
// there are no items.
func (p *ProgressList) Start() {
	if !p.animatable() || len(p.items) == 0 {
		return
	}
	p.mu.Lock()
	p.anim = true
	p.running = true
	p.stop = make(chan struct{})
	p.done = make(chan struct{})
	p.mu.Unlock()
	go p.run()
	p.redraw(string(frames[0]))
}

// Advance marks the current item done and moves the spinner to the next item.
// It is a no-op when rendering is off.
func (p *ProgressList) Advance() {
	p.mu.Lock()
	anim := p.anim
	if anim {
		p.active++
	}
	p.mu.Unlock()
	if anim {
		p.redraw("")
	}
}

// Finish stops the animation and leaves the final list on screen.
func (p *ProgressList) Finish() {
	p.mu.Lock()
	if !p.running {
		p.mu.Unlock()
		return
	}
	p.running = false
	close(p.stop)
	p.mu.Unlock()
	<-p.done
	p.redraw("")
	p.mu.Lock()
	p.anim = false
	p.mu.Unlock()
}

func (p *ProgressList) run() {
	defer close(p.done)
	i := 0
	for {
		select {
		case <-p.stop:
			return
		case <-time.After(refresh):
		}
		p.redraw(string(frames[i%len(frames)]))
		i++
	}
}

func (p *ProgressList) redraw(frame string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if !p.anim || len(p.items) == 0 {
		return
	}
	if p.drawn {
		fmt.Fprintf(p.w, "\x1b[%dA", len(p.items))
	}
	p.drawn = true
	for i, label := range p.items {
		fmt.Fprint(p.w, "\r\x1b[2K")
		switch {
		case i < p.active:
			fmt.Fprintf(p.w, "✓ %s\n", label)
		case i == p.active && frame != "":
			fmt.Fprintf(p.w, "%s %s\n", frame, label)
		default:
			fmt.Fprintf(p.w, "  %s\n", label)
		}
	}
}

func (p *ProgressList) animatable() bool {
	f, ok := p.w.(interface{ Stat() (os.FileInfo, error) })
	if !ok {
		return false
	}
	fi, err := f.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}
