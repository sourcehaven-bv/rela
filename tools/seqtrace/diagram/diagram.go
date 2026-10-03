// Package diagram turns a seqtrace event log into a tree of steps per
// request ([Build]), and renders it as a Mermaid sequence diagram
// ([Diagram.Mermaid]) or an indented text tree ([Diagram.Text]). [Compare]
// reports how the trees of two runs differ.
//
// Participants are packages. A call inside one package is not drawn; its
// cross-package calls are drawn from that package. A call whose parent runs
// on another goroutine is drawn as an async arrow labeled with how the
// parent was found (go, ctx, closure). Repeats fold into loops; see fold.
//
// Call arrows carry the argument summaries the runtime recorded and return
// arrows the result summaries. Repeats that differ only in those values
// still fold; the loop then shows "…" in place of the values.
package diagram

import (
	"bufio"
	"encoding/json"
	"io"
	"regexp"
	"sort"
)

// Event mirrors seqtrace.Event. It is copied rather than imported because
// importing the runtime runs its init, which truncates SEQTRACE_OUT when
// that variable is set in the converter's environment.
type Event struct {
	Kind   string `json:"e"`
	ID     uint64 `json:"id"`
	Parent uint64 `json:"p"`
	Via    string `json:"v"`
	G      uint64 `json:"g"`
	Pkg    string `json:"pkg"`
	Fn     string `json:"fn"`
	T      int64  `json:"t"`
	// Args are the call's argument summaries; Results, copied from the
	// matching return event, its result summaries.
	Args    []string `json:"a"`
	Results []string `json:"rv"`
}

// Call is one recorded call with its children, ordered by start time.
type Call struct {
	Event
	Children []*Call
}

// Title returns the most specific call matching re on the path down from
// c, so a diagram can be named after its HTTP handler rather than the
// outermost middleware closure. It returns c when nothing matches.
func (c *Call) Title(re *regexp.Regexp) *Call {
	best := c
	for {
		next := best.firstMatch(re)
		if next == nil {
			return best
		}
		best = next
	}
}

// firstMatch is the first descendant of c, in call order, matching re.
func (c *Call) firstMatch(re *regexp.Regexp) *Call {
	for _, ch := range c.Children {
		if re.MatchString(ch.Fn) {
			return ch
		}
		if m := ch.firstMatch(re); m != nil {
			return m
		}
	}
	return nil
}

// Name is the label the runtime matched SEQTRACE_ROOT against.
func (c *Call) Name() string { return c.Pkg + "." + c.Fn }

const (
	initLineBuf = 1 << 16
	maxLine     = 1 << 20
)

// Read parses a trace file and returns its root calls in start order.
// A call whose parent is missing from the file is treated as a root.
func Read(r io.Reader) ([]*Call, error) {
	calls := map[uint64]*Call{}
	var order []*Call
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, initLineBuf), maxLine)
	for sc.Scan() {
		var e Event
		if err := json.Unmarshal(sc.Bytes(), &e); err != nil {
			// The last line is often cut short when the process is killed.
			continue
		}
		if e.Kind == "r" {
			if c := calls[e.ID]; c != nil {
				c.Results = e.Results
			}
			continue
		}
		if e.Kind != "c" {
			continue
		}
		c := &Call{Event: e}
		calls[e.ID] = c
		order = append(order, c)
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	var roots []*Call
	for _, c := range order {
		if p := calls[c.Parent]; c.Parent != 0 && p != nil {
			p.Children = append(p.Children, c)
		} else {
			roots = append(roots, c)
		}
	}
	for _, c := range order {
		sort.SliceStable(c.Children, func(i, j int) bool { return c.Children[i].T < c.Children[j].T })
	}
	return roots, nil
}

// Options control rendering.
type Options struct {
	// Collapse matches package labels whose calls are drawn as if they were
	// part of the caller's package. Use it for value packages (entity, ...)
	// that add arrows without adding understanding.
	Collapse *regexp.Regexp
	// MaxDepth limits nesting of drawn arrows; zero means no limit.
	MaxDepth int
	// RootLabel replaces the root call's name on the first arrow.
	RootLabel string
	// Values draws argument and result summaries on the arrows.
	Values bool
}
