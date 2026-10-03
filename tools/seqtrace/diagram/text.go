package diagram

import (
	"fmt"
	"io"
	"strings"
	"unicode"
)

// Text writes d as an indented call tree, one line per call:
//
//	dataentry handleV1GetEntity(r=*http.Request)
//	  acl (*Request).PermitsRead(entityType="task") -> true
//	  2× store/pgstore (*Store).ListRelations(…) -> …
//	  ~go jobs run
//
// A child is indented under its caller, "-> " gives the results, "N×" a loop
// and "~via" a call on another goroutine. It carries what the diagram does at
// a fraction of the size, for agents and diffs.
func (d *Diagram) Text(w io.Writer) error {
	_, err := io.WriteString(w, strings.Join(d.lines(false), "\n")+"\n")
	return err
}

// Shape returns the text lines with every value replaced by "…", so runs
// that differ only in IDs compare equal.
func (d *Diagram) Shape() []string { return d.lines(true) }

func (d *Diagram) lines(shape bool) []string {
	return textSteps([]Step{d.Root}, 0, shape, "")
}

func textSteps(steps []Step, depth int, elide bool, prefix string) []string {
	var out []string
	for i := range steps {
		s := &steps[i]
		pre := prefix
		if i > 0 {
			pre = ""
		}
		if s.IsLoop() {
			rep := fmt.Sprintf("%d× ", s.Repeat)
			if len(s.Children) == 1 {
				out = append(out, textSteps(s.Children, depth, elide || s.Elided, pre+rep)...)
				continue
			}
			out = append(out, strings.Repeat("  ", depth)+pre+rep+"loop")
			out = append(out, textSteps(s.Children, depth+1, elide || s.Elided, "")...)
			continue
		}
		var b strings.Builder
		b.WriteString(strings.Repeat("  ", depth))
		b.WriteString(pre)
		if s.Via != "" {
			b.WriteString("~" + s.Via + " ")
		}
		b.WriteString(s.To + " " + s.Fn)
		if len(s.Args) > 0 {
			if elide {
				b.WriteString("(…)")
			} else {
				b.WriteString("(" + oneLine(strings.Join(s.Args, ", ")) + ")")
			}
		}
		if len(s.Results) > 0 {
			if elide {
				b.WriteString(" -> …")
			} else {
				b.WriteString(" -> " + oneLine(strings.Join(s.Results, ", ")))
			}
		}
		out = append(out, b.String())
		out = append(out, textSteps(s.Children, depth+1, elide, "")...)
	}
	return out
}

// oneLine replaces control characters with spaces. Values can come from
// user content, and a raw escape sequence would act on the terminal that
// prints the tree.
func oneLine(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return ' '
		}
		return r
	}, s)
}
