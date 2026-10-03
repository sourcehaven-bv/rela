package diagram

import (
	"fmt"
	"io"
	"sort"
	"strings"
)

// Scenario is one rendered request, as stored in a diagram directory's
// steps.json so two runs can be compared.
type Scenario struct {
	Name    string   `json:"name,omitempty"` // from -names; may be empty
	Handler string   `json:"handler"`
	Base    string   `json:"base"` // output file name without extension
	Diagram *Diagram `json:"diagram"`
}

// keys matches scenarios across runs: by name, or by handler and its
// occurrence among unnamed scenarios.
func keys(ss []Scenario) []string {
	seen := map[string]int{}
	out := make([]string, len(ss))
	for i, s := range ss {
		k := s.Name
		if k == "" {
			seen[s.Handler]++
			k = fmt.Sprintf("%s #%d", s.Handler, seen[s.Handler])
		}
		out[i] = k
	}
	return out
}

// diffContext is the number of unchanged lines shown around a change.
const diffContext = 2

// maxDiffCells bounds the line-diff table; larger changes are summarized.
const maxDiffCells = 4_000_000

// Compare writes a report of how the scenarios in head differ from those in
// base and returns the number that changed, were added or were removed.
// With values false, calls are compared by shape only.
func Compare(w io.Writer, base, head []Scenario, values bool) (int, error) {
	render := func(d *Diagram) []string {
		if values {
			return d.lines(false)
		}
		return d.Shape()
	}
	bk, hk := keys(base), keys(head)
	byKey := map[string]*Scenario{}
	for i := range base {
		byKey[bk[i]] = &base[i]
	}
	var b strings.Builder
	changed, unchanged := 0, 0
	for i := range head {
		h := &head[i]
		old, ok := byKey[hk[i]]
		if !ok {
			changed++
			fmt.Fprintf(&b, "## %s: added (%d calls)\n\n", hk[i], h.Diagram.Calls())
			continue
		}
		delete(byKey, hk[i])
		ol, nl := render(old.Diagram), render(h.Diagram)
		if equalLines(ol, nl) {
			unchanged++
			continue
		}
		changed++
		fmt.Fprintf(&b, "## %s: changed\n\ncalls: %d -> %d\n", hk[i], old.Diagram.Calls(), h.Diagram.Calls())
		writeEdgeDeltas(&b, old.Diagram.Edges(), h.Diagram.Edges())
		b.WriteString("\n```diff\n")
		writeLineDiff(&b, ol, nl)
		b.WriteString("```\n\n")
	}
	for _, k := range bk {
		if s, ok := byKey[k]; ok {
			changed++
			fmt.Fprintf(&b, "## %s: removed (%d calls)\n\n", k, s.Diagram.Calls())
		}
	}
	fmt.Fprintf(&b, "%d changed, %d unchanged\n", changed, unchanged)
	_, err := io.WriteString(w, b.String())
	return changed, err
}

func writeEdgeDeltas(b *strings.Builder, old, cur map[string]int) {
	var ks []string
	for k := range old {
		ks = append(ks, k)
	}
	for k := range cur {
		if _, ok := old[k]; !ok {
			ks = append(ks, k)
		}
	}
	sort.Strings(ks)
	for _, k := range ks {
		o, n := old[k], cur[k]
		switch {
		case o == n:
		case o == 0:
			fmt.Fprintf(b, "+ %s: %d (new edge)\n", k, n)
		case n == 0:
			fmt.Fprintf(b, "- %s: %d (edge gone)\n", k, o)
		default:
			fmt.Fprintf(b, "~ %s: %d -> %d\n", k, o, n)
		}
	}
}

func equalLines(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

type op struct {
	kind byte // ' ', '-' or '+'
	line string
}

// writeLineDiff writes a unified-style diff of a and b with diffContext
// lines of context, using a longest-common-subsequence table over the part
// between the common prefix and suffix.
func writeLineDiff(b *strings.Builder, a, c []string) {
	pre := 0
	for pre < len(a) && pre < len(c) && a[pre] == c[pre] {
		pre++
	}
	suf := 0
	for suf < len(a)-pre && suf < len(c)-pre && a[len(a)-1-suf] == c[len(c)-1-suf] {
		suf++
	}
	ops := make([]op, 0, len(a)+len(c))
	for _, l := range a[:pre] {
		ops = append(ops, op{' ', l})
	}
	ops = append(ops, middleOps(a[pre:len(a)-suf], c[pre:len(c)-suf])...)
	for _, l := range a[len(a)-suf:] {
		ops = append(ops, op{' ', l})
	}

	// Print changes with context, separating distant hunks.
	show := make([]bool, len(ops))
	for i, o := range ops {
		if o.kind == ' ' {
			continue
		}
		for j := max(0, i-diffContext); j <= min(len(ops)-1, i+diffContext); j++ {
			show[j] = true
		}
	}
	gap := false
	for i, o := range ops {
		if !show[i] {
			gap = true
			continue
		}
		if gap {
			b.WriteString("@@\n")
			gap = false
		}
		b.WriteByte(o.kind)
		b.WriteString(o.line)
		b.WriteByte('\n')
	}
}

func middleOps(a, c []string) []op {
	if len(a)*len(c) > maxDiffCells {
		ops := make([]op, 0, len(a)+len(c))
		for _, l := range a {
			ops = append(ops, op{'-', l})
		}
		for _, l := range c {
			ops = append(ops, op{'+', l})
		}
		return ops
	}
	// lcs[i][j] is the LCS length of a[i:] and c[j:].
	lcs := make([][]int, len(a)+1)
	for i := range lcs {
		lcs[i] = make([]int, len(c)+1)
	}
	for i := len(a) - 1; i >= 0; i-- {
		for j := len(c) - 1; j >= 0; j-- {
			if a[i] == c[j] {
				lcs[i][j] = lcs[i+1][j+1] + 1
			} else {
				lcs[i][j] = max(lcs[i+1][j], lcs[i][j+1])
			}
		}
	}
	var ops []op
	i, j := 0, 0
	for i < len(a) && j < len(c) {
		switch {
		case a[i] == c[j]:
			ops = append(ops, op{' ', a[i]})
			i++
			j++
		case lcs[i+1][j] >= lcs[i][j+1]:
			ops = append(ops, op{'-', a[i]})
			i++
		default:
			ops = append(ops, op{'+', c[j]})
			j++
		}
	}
	for ; i < len(a); i++ {
		ops = append(ops, op{'-', a[i]})
	}
	for ; j < len(c); j++ {
		ops = append(ops, op{'+', c[j]})
	}
	return ops
}
