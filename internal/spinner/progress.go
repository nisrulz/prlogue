package spinner

import (
	"fmt"
	"io"
	"sync"
	"time"
)

// ProgressList renders one permanent tick line per completed item and a live
// status line for the item being processed. It animates only when the writer is
// a terminal; any other writer gets one tick line per completed item. It never
// emits cursor-movement escapes, so it stays clean in multiplexers and panes
// that do not render cursor-up.
type ProgressList struct {
	w     io.Writer
	items []string

	mu      sync.Mutex
	active  int
	frame   string
	stop    chan struct{}
	done    chan struct{}
	anim    bool
	static  bool
	running bool
}

// NewProgressList returns a progress list that writes to w.
func NewProgressList(w io.Writer, items []string) *ProgressList {
	return &ProgressList{w: w, items: items}
}

// Start begins rendering the list. It animates when w is a terminal; for any
// other writer it marks each item done as Advance is called. It is a no-op
// when there are no items.
func (p *ProgressList) Start() {
	if len(p.items) == 0 {
		return
	}
	if !animatable(p.w) {
		p.static = true
		return
	}
	p.mu.Lock()
	p.anim = true
	p.running = true
	p.frame = string(frames[0])
	p.stop = make(chan struct{})
	p.done = make(chan struct{})
	fmt.Fprintf(p.w, "\r%s %s", p.frame, p.items[0])
	p.mu.Unlock()
	go p.run()
}

// Advance marks the current item done: it prints the tick line and moves the
// status line to the next item. When rendering is off, it prints the completed
// item as a tick line.
func (p *ProgressList) Advance() {
	if p.static {
		if p.active < len(p.items) {
			fmt.Fprintf(p.w, "✓ %s\n", p.items[p.active])
		}
		p.active++
		return
	}
	p.mu.Lock()
	anim := p.anim
	if anim {
		p.active++
	}
	p.mu.Unlock()
	if anim {
		p.redraw()
	}
}

// Finish stops the animation and clears any pending status line.
func (p *ProgressList) Finish() {
	if p.static {
		return
	}
	p.mu.Lock()
	if !p.running {
		p.mu.Unlock()
		return
	}
	p.running = false
	close(p.stop)
	p.mu.Unlock()
	<-p.done
	p.mu.Lock()
	p.anim = false
	if p.active < len(p.items) {
		fmt.Fprint(p.w, "\r\x1b[2K")
	}
	p.mu.Unlock()
}

func (p *ProgressList) run() {
	defer close(p.done)
	for i := 1; ; i++ {
		select {
		case <-p.stop:
			return
		case <-time.After(refresh):
		}
		p.mu.Lock()
		if p.anim && p.active < len(p.items) {
			p.frame = string(frames[i%len(frames)])
			fmt.Fprintf(p.w, "\r%s %s", p.frame, p.items[p.active])
		}
		p.mu.Unlock()
	}
}

// redraw prints the tick line for the just-finished item, then draws the status
// line for the next pending item, or clears the line when all items are done.
func (p *ProgressList) redraw() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if !p.anim {
		return
	}
	finished := p.active - 1
	if finished >= 0 && finished < len(p.items) {
		fmt.Fprintf(p.w, "\r\x1b[2K✓ %s\n", p.items[finished])
	}
	if p.active < len(p.items) {
		fmt.Fprintf(p.w, "\r%s %s", p.frame, p.items[p.active])
	} else {
		fmt.Fprint(p.w, "\r\x1b[2K")
	}
}
