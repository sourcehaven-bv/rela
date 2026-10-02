// Package diagram turns a seqtrace event log into Mermaid sequence diagrams.
//
// Participants are packages. A call inside one package is not drawn; its
// cross-package calls are drawn from that package. A call whose parent runs
// on another goroutine is drawn as an async arrow labeled with how the
// parent was found (go, ctx, closure). Consecutive identical sibling calls
// fold into a Mermaid loop block.
//
// Call arrows carry the argument summaries the runtime recorded and return
// arrows the result summaries. Siblings that differ only in those values
// still fold; the loop then shows "…" in place of the values.
package diagram

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"
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

// maxLabel caps an arrow label, so one long value does not stretch the
// diagram. Mermaid wraps what remains.
const maxLabel = 80

// Render writes one Mermaid sequence diagram for root. It returns the
// number of arrows drawn, so callers can skip trivial diagrams.
func Render(w io.Writer, root *Call, opt Options) (int, error) {
	r := &renderer{opt: opt, parts: map[string]bool{}}
	rootPart := r.part(root.Pkg)
	inner := r.children(root, rootPart, 1)
	first := root.Fn
	if opt.RootLabel != "" {
		first = opt.RootLabel
	}
	body := make([]line, 0, len(inner)+2)
	body = append(body, r.callLine("Caller", rootPart, "->>+", first, root))
	body = append(body, inner...)
	body = append(body, r.returnLine(rootPart, "Caller", root))

	var b strings.Builder
	b.WriteString("sequenceDiagram\n    autonumber\n    actor Caller\n")
	for _, p := range r.partOrder {
		fmt.Fprintf(&b, "    participant %s as %s\n", mid(p), p)
	}
	arrows := 0
	for _, l := range body {
		b.WriteString("    ")
		b.WriteString(l.text)
		b.WriteByte('\n')
		if l.arrow {
			arrows++
		}
	}
	_, err := io.WriteString(w, b.String())
	return arrows, err
}

// line is one line of the diagram body. shape is the line with values
// replaced by "…"; siblings with equal shapes fold into one loop.
type line struct {
	text, shape string
	arrow       bool // a call arrow, counted by Render
}

type renderer struct {
	opt       Options
	parts     map[string]bool
	partOrder []string
}

// part returns the participant a package is drawn as, registering it in
// first-seen order so the diagram reads left to right.
func (r *renderer) part(pkg string) string {
	p := strings.TrimPrefix(pkg, "internal/")
	if !r.parts[p] {
		r.parts[p] = true
		r.partOrder = append(r.partOrder, p)
	}
	return p
}

// children renders the calls under c as seen from participant from.
// Consecutive renderings with the same shape fold into a loop block, which
// shows the values only when every repetition had the same ones.
func (r *renderer) children(c *Call, from string, depth int) []line {
	var out, prev []line
	count, same := 0, true
	flush := func() {
		switch {
		case count == 1:
			out = append(out, prev...)
		case count > 1:
			head := fmt.Sprintf("loop %d times", count)
			out = append(out, line{text: head, shape: head})
			for _, l := range prev {
				text := l.text
				if !same {
					text = l.shape
				}
				out = append(out, line{text: "    " + text, shape: "    " + l.shape, arrow: l.arrow})
			}
			out = append(out, line{text: "end", shape: "end"})
		}
	}
	for _, ch := range c.Children {
		lines := r.call(ch, from, depth)
		if len(lines) == 0 {
			continue
		}
		if count > 0 && equal(lines, prev, func(l line) string { return l.shape }) {
			count++
			same = same && equal(lines, prev, func(l line) string { return l.text })
			continue
		}
		flush()
		prev, count, same = lines, 1, true
	}
	flush()
	return out
}

func (r *renderer) call(c *Call, from string, depth int) []line {
	collapsed := r.opt.Collapse != nil && r.opt.Collapse.MatchString(c.Pkg)
	to := from
	if !collapsed {
		to = r.part(c.Pkg)
	}
	if to == from && c.Via == "" {
		return r.children(c, from, depth)
	}
	if r.opt.MaxDepth > 0 && depth >= r.opt.MaxDepth {
		return nil
	}
	if c.Via != "" {
		return append([]line{r.callLine(from, to, "-)", "["+c.Via+"] "+c.Fn, c)}, r.children(c, to, depth+1)...)
	}
	out := []line{r.callLine(from, to, "->>+", c.Fn, c)}
	out = append(out, r.children(c, to, depth+1)...)
	return append(out, r.returnLine(to, from, c))
}

// callLine draws a call arrow labeled name(args).
func (r *renderer) callLine(from, to, arrow, name string, c *Call) line {
	prefix := mid(from) + arrow + mid(to) + ": "
	if !r.opt.Values || len(c.Args) == 0 {
		return line{text: prefix + label(name), shape: prefix + label(name), arrow: true}
	}
	text := name + "(" + strings.Join(c.Args, ", ") + ")"
	return line{text: prefix + label(capLabel(text)), shape: prefix + label(name+"(…)"), arrow: true}
}

// returnLine draws a return arrow labeled with the results. A trailing nil
// after other results is dropped: it is almost always the error.
func (r *renderer) returnLine(from, to string, c *Call) line {
	prefix := mid(from) + "-->>-" + mid(to) + ": "
	rv := c.Results
	if len(rv) > 1 && rv[len(rv)-1] == "nil" {
		rv = rv[:len(rv)-1]
	}
	if !r.opt.Values || len(rv) == 0 {
		return line{text: prefix, shape: prefix}
	}
	return line{text: prefix + label(capLabel(strings.Join(rv, ", "))), shape: prefix + "…"}
}

func capLabel(s string) string {
	if len(s) <= maxLabel {
		return s
	}
	cut := maxLabel
	for cut > 0 && !utf8.RuneStart(s[cut]) {
		cut--
	}
	return s[:cut] + "…"
}

func equal(a, b []line, key func(line) string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if key(a[i]) != key(b[i]) {
			return false
		}
	}
	return true
}

// mid turns a participant label into a Mermaid identifier.
func mid(p string) string {
	return strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' {
			return r
		}
		return '_'
	}, p)
}

// label escapes characters Mermaid treats as syntax in a message. Angle
// brackets would otherwise be read as HTML, a line break would end the
// message, and a backtick could close the Markdown fence.
func label(s string) string {
	return strings.NewReplacer(";", "#59;", "#", "#35;", "<", "#60;", ">", "#62;",
		"`", "#96;", "\r", " ", "\n", " ", "\x00", " ").Replace(s)
}
