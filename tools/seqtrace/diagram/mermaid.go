package diagram

import (
	"fmt"
	"io"
	"strings"
	"unicode/utf8"
)

// maxLabel caps an arrow label, so one long value does not stretch the
// diagram. Mermaid wraps what remains.
const maxLabel = 80

// Mermaid writes d as a Mermaid sequence diagram.
func (d *Diagram) Mermaid(w io.Writer) error {
	var b strings.Builder
	b.WriteString("sequenceDiagram\n    autonumber\n    actor Caller\n")
	for _, p := range d.Participants {
		fmt.Fprintf(&b, "    participant %s as %s\n", mid(p), p)
	}
	for _, l := range mermaidSteps([]Step{d.Root}, false) {
		b.WriteString("    ")
		b.WriteString(l)
		b.WriteByte('\n')
	}
	_, err := io.WriteString(w, b.String())
	return err
}

// mermaidSteps renders steps; elide shows "…" in place of values.
func mermaidSteps(steps []Step, elide bool) []string {
	var out []string
	for i := range steps {
		s := &steps[i]
		switch {
		case s.IsLoop():
			out = append(out, fmt.Sprintf("loop %d times", s.Repeat))
			for _, l := range mermaidSteps(s.Children, elide || s.Elided) {
				out = append(out, "    "+l)
			}
			out = append(out, "end")
		case s.Via != "":
			out = append(out, mid(s.From)+"-)"+mid(s.To)+": "+callLabel("["+s.Via+"] "+s.Fn, s.Args, elide))
			out = append(out, mermaidSteps(s.Children, elide)...)
		default:
			out = append(out, mid(s.From)+"->>+"+mid(s.To)+": "+callLabel(s.Fn, s.Args, elide))
			out = append(out, mermaidSteps(s.Children, elide)...)
			out = append(out, mid(s.To)+"-->>-"+mid(s.From)+": "+resultLabel(s.Results, elide))
		}
	}
	return out
}

func callLabel(name string, args []string, elide bool) string {
	switch {
	case len(args) == 0:
		return label(name)
	case elide:
		return label(name + "(…)")
	}
	return label(capLabel(name + "(" + strings.Join(args, ", ") + ")"))
}

func resultLabel(rv []string, elide bool) string {
	switch {
	case len(rv) == 0:
		return ""
	case elide:
		return "…"
	}
	return label(capLabel(strings.Join(rv, ", ")))
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
