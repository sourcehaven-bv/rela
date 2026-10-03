package diagram

import (
	"regexp"
	"strings"
	"testing"
)

func buildTrace(t *testing.T) *Diagram {
	t.Helper()
	roots, err := Read(strings.NewReader(trace))
	if err != nil {
		t.Fatal(err)
	}
	return Build(roots[0], Options{Values: true, Collapse: regexp.MustCompile(`^internal/entity$`)})
}

func TestRenderText(t *testing.T) {
	d := buildTrace(t)
	var b strings.Builder
	if err := d.Text(&b); err != nil {
		t.Fatal(err)
	}
	want := `dataentry handle
  2× store Get(…) -> …
  audit Record(op="<update>;") -> err: disk full
  ~go jobs run
`
	if b.String() != want {
		t.Errorf("Text =\n%s\nwant\n%s", b.String(), want)
	}
	if got := d.Calls(); got != 5 {
		t.Errorf("Calls = %d, want 5 (the loop counts twice)", got)
	}
	if got := d.Arrows(); got != 4 {
		t.Errorf("Arrows = %d, want 4", got)
	}
	edges := d.Edges()
	if edges["dataentry -> store"] != 2 || edges["dataentry -> jobs"] != 1 || edges["Caller -> dataentry"] != 1 {
		t.Errorf("Edges = %v", edges)
	}
}

func TestRenderTextMultiStepLoop(t *testing.T) {
	d := &Diagram{Root: Step{From: "Caller", To: "a", Fn: "F", Children: []Step{
		{Repeat: 3, Children: []Step{{From: "a", To: "b", Fn: "G"}, {From: "a", To: "c", Fn: "H"}}},
	}}}
	want := []string{"a F", "  3× loop", "    b G", "    c H"}
	if got := d.Shape(); strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("Shape = %q, want %q", got, want)
	}
}

func TestCompare(t *testing.T) {
	base := buildTrace(t)
	head := buildTrace(t)
	// head: one more Get (a value-only change elsewhere must not count), and
	// a new call to visibility.
	head.Root.Children[0].Repeat = 3
	head.Root.Children[1].Args = []string{`op="<delete>"`}
	head.Root.Children = append(head.Root.Children, Step{From: "dataentry", To: "visibility", Fn: "Redact"})

	same := []Scenario{{Name: "same", Handler: "h", Diagram: buildTrace(t)}}
	var b strings.Builder
	n, err := Compare(&b,
		append([]Scenario{{Name: "read", Diagram: base}, {Name: "gone", Diagram: base}}, same...),
		append([]Scenario{{Name: "read", Diagram: head}, {Handler: "new", Diagram: head}}, same...),
		false)
	if err != nil {
		t.Fatal(err)
	}
	got := b.String()
	for _, want := range []string{
		"## read: changed\n\ncalls: 5 -> 7\n",
		"+ dataentry -> visibility: 1 (new edge)\n",
		"~ dataentry -> store: 2 -> 3\n",
		"-  2× store Get(…) -> …\n+  3× store Get(…) -> …\n",
		"+  visibility Redact\n",
		"## new #1: added (7 calls)",
		"## gone: removed (5 calls)",
		"3 changed, 1 unchanged",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in:\n%s", want, got)
		}
	}
	if strings.Contains(got, "<delete>") {
		t.Errorf("shape comparison showed a value:\n%s", got)
	}
	if n != 3 {
		t.Errorf("changed = %d, want 3", n)
	}
}

func TestLineDiffContext(t *testing.T) {
	a := []string{"1", "2", "3", "4", "5", "6", "7", "8", "9"}
	c := []string{"1", "x", "3", "4", "5", "6", "7", "8", "y"}
	var b strings.Builder
	writeLineDiff(&b, a, c)
	want := " 1\n-2\n+x\n 3\n 4\n@@\n 7\n 8\n-9\n+y\n"
	if b.String() != want {
		t.Errorf("diff =\n%s\nwant\n%s", b.String(), want)
	}
}

func TestFoldPeriodic(t *testing.T) {
	st := func(to, arg string) Step { return Step{From: "a", To: to, Fn: "F", Args: []string{arg}} }
	steps := []Step{
		st("x", "0"),
		st("b", "1"), st("c", "1"), st("d", "1"),
		st("b", "2"), st("c", "2"), st("d", "2"),
		st("b", "3"), st("c", "3"), st("d", "3"),
		st("e", "0"), st("e", "0"),
	}
	d := &Diagram{Root: Step{From: "Caller", To: "a", Fn: "H", Children: fold(steps)}}
	got := strings.Join(d.lines(false), "\n")
	exp := strings.Join([]string{
		"a H",
		"  x F(0)",
		"  3× loop",
		"    b F(…)",
		"    c F(…)",
		"    d F(…)",
		"  2× e F(0)",
	}, "\n")
	if got != exp {
		t.Errorf("folded =\n%s\nwant\n%s", got, exp)
	}
}
