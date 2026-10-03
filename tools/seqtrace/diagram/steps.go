package diagram

import (
	"reflect"
	"strings"
)

// Diagram is a call tree reduced to what the renderers draw: calls between
// participants, with same-package calls removed, collapsed packages merged
// into their callers, and repeats folded into loops.
type Diagram struct {
	Participants []string `json:"participants"`
	Root         Step     `json:"root"`
}

// Step is one drawn call, or a loop when Repeat is set. A loop has no
// endpoints; its Children are one iteration.
type Step struct {
	From    string   `json:"from,omitempty"`
	To      string   `json:"to,omitempty"`
	Fn      string   `json:"fn,omitempty"`
	Via     string   `json:"via,omitempty"` // set for a call on another goroutine
	Args    []string `json:"args,omitempty"`
	Results []string `json:"results,omitempty"`
	Repeat  int      `json:"repeat,omitempty"`
	// Elided marks a loop whose iterations differed only in values; the
	// renderers then show "…" in place of the values.
	Elided   bool   `json:"elided,omitempty"`
	Children []Step `json:"children,omitempty"`
}

// IsLoop reports whether s is a loop rather than a call.
func (s *Step) IsLoop() bool { return s.Repeat > 0 }

// Build reduces the call tree under root to a Diagram.
func Build(root *Call, opt Options) *Diagram {
	b := &builder{opt: opt, parts: map[string]bool{}}
	rootPart := b.part(root.Pkg)
	fn := root.Fn
	if opt.RootLabel != "" {
		fn = opt.RootLabel
	}
	top := b.step("Caller", rootPart, fn, root)
	top.Children = b.children(root, rootPart, 1)
	return &Diagram{Participants: b.partOrder, Root: top}
}

// Arrows counts the drawn calls, counting a loop's body once.
func (d *Diagram) Arrows() int { return countCalls(d.Root, false) }

// Calls counts the calls made, multiplying loop bodies by their repeats.
func (d *Diagram) Calls() int { return countCalls(d.Root, true) }

func countCalls(s Step, expand bool) int {
	n := 0
	if !s.IsLoop() {
		n = 1
	}
	inner := 0
	for _, c := range s.Children {
		inner += countCalls(c, expand)
	}
	if expand && s.IsLoop() {
		inner *= s.Repeat
	}
	return n + inner
}

// Edges counts the calls made from one participant to another, keyed
// "from -> to", with loop bodies multiplied by their repeats.
func (d *Diagram) Edges() map[string]int {
	out := map[string]int{}
	var walk func(s Step, mult int)
	walk = func(s Step, mult int) {
		if s.IsLoop() {
			mult *= s.Repeat
		} else {
			out[s.From+" -> "+s.To] += mult
		}
		for _, c := range s.Children {
			walk(c, mult)
		}
	}
	walk(d.Root, 1)
	return out
}

type builder struct {
	opt       Options
	parts     map[string]bool
	partOrder []string
}

// part returns the participant a package is drawn as, registering it in
// first-seen order so the diagram reads left to right.
func (b *builder) part(pkg string) string {
	p := strings.TrimPrefix(pkg, "internal/")
	if !b.parts[p] {
		b.parts[p] = true
		b.partOrder = append(b.partOrder, p)
	}
	return p
}

// step makes the call step for c. A trailing nil result after other
// results is dropped: it is almost always the error.
func (b *builder) step(from, to, fn string, c *Call) Step {
	s := Step{From: from, To: to, Fn: fn, Via: c.Via}
	if !b.opt.Values {
		return s
	}
	s.Args = c.Args
	rv := c.Results
	if len(rv) > 1 && rv[len(rv)-1] == "nil" {
		rv = rv[:len(rv)-1]
	}
	s.Results = rv
	return s
}

// children builds the steps under c as seen from participant from.
// Consecutive groups with the same shape fold into a loop.
func (b *builder) children(c *Call, from string, depth int) []Step {
	flat := make([]Step, 0, len(c.Children))
	for _, ch := range c.Children {
		flat = append(flat, b.call(ch, from, depth)...)
	}
	return fold(flat)
}

// maxPeriod is the longest run of steps fold recognizes as a loop body.
const maxPeriod = 8

// fold replaces back-to-back repeats of a run of 1 to maxPeriod steps with
// a loop step. A loop in one function that calls three others shows up as a
// repeating run of three, so matching single steps only would miss it. At
// each position it takes the period that covers the most steps, the shortest
// on a tie.
func fold(steps []Step) []Step {
	var out []Step
	for i := 0; i < len(steps); {
		bestK, bestN := 1, 1
		for k := 1; k <= maxPeriod && i+2*k <= len(steps); k++ {
			n := 1
			for i+(n+1)*k <= len(steps) && sameShape(steps[i+n*k:i+(n+1)*k], steps[i:i+k]) {
				n++
			}
			if n > 1 && n*k > bestN*bestK {
				bestK, bestN = k, n
			}
		}
		if bestN == 1 {
			out = append(out, steps[i])
			i++
			continue
		}
		body := steps[i : i+bestK]
		same := true
		for j := 1; j < bestN && same; j++ {
			same = reflect.DeepEqual(steps[i+j*bestK:i+(j+1)*bestK], body)
		}
		out = append(out, Step{Repeat: bestN, Elided: !same, Children: body})
		i += bestN * bestK
	}
	return out
}

// call returns the steps c contributes: one step, or, for a call inside the
// same participant, the steps of its children.
func (b *builder) call(c *Call, from string, depth int) []Step {
	to := from
	if b.opt.Collapse == nil || !b.opt.Collapse.MatchString(c.Pkg) {
		to = b.part(c.Pkg)
	}
	if to == from && c.Via == "" {
		return b.children(c, from, depth)
	}
	if b.opt.MaxDepth > 0 && depth >= b.opt.MaxDepth {
		return nil
	}
	s := b.step(from, to, c.Fn, c)
	s.Children = b.children(c, to, depth+1)
	return []Step{s}
}

// sameShape compares step lists ignoring values, though not whether a
// call had arguments or results.
func sameShape(a, b []Step) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		x, y := &a[i], &b[i]
		if x.From != y.From || x.To != y.To || x.Fn != y.Fn || x.Via != y.Via || x.Repeat != y.Repeat ||
			(len(x.Args) > 0) != (len(y.Args) > 0) || (len(x.Results) > 0) != (len(y.Results) > 0) ||
			!sameShape(x.Children, y.Children) {

			return false
		}
	}
	return true
}
