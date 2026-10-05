package diagram

import (
	"regexp"
	"strings"
	"testing"
	"unicode/utf8"
)

const trace = `{"e":"c","id":1,"g":1,"pkg":"internal/dataentry","fn":"handle","t":0}
{"e":"c","id":2,"p":1,"g":1,"pkg":"internal/dataentry","fn":"helper","t":1000}
{"e":"c","id":3,"p":2,"g":1,"pkg":"internal/store","fn":"Get","t":2000,"a":["id=\"T-1\""]}
{"e":"r","id":3,"t":3000,"rv":["*entity.Entity{ID:T-1}","nil"]}
{"e":"c","id":4,"p":2,"g":1,"pkg":"internal/store","fn":"Get","t":4000,"a":["id=\"T-2\""]}
{"e":"r","id":4,"t":6000,"rv":["*entity.Entity{ID:T-2}","nil"]}
{"e":"r","id":2,"t":7000}
{"e":"c","id":5,"p":1,"g":1,"pkg":"internal/entity","fn":"Clone","t":8000}
{"e":"c","id":6,"p":5,"g":1,"pkg":"internal/audit","fn":"Record","t":9000,"a":["op=\"<update>;\""]}
{"e":"r","id":6,"t":10000,"rv":["err: disk full"]}
{"e":"r","id":5,"t":11000}
{"e":"c","id":7,"p":1,"v":"go","g":2,"pkg":"internal/jobs","fn":"run","t":12000}
{"e":"r","id":1,"t":20000}
{"e":"c","id":8,"p":99,"g":3,"pkg":"internal/other","fn":"orphan","t":21000}
{"e":"c","id":9,"p":7,"g":2,"pkg":"internal/jobs","fn":"cut short`

func TestRender(t *testing.T) {
	roots, err := Read(strings.NewReader(trace))
	if err != nil {
		t.Fatal(err)
	}
	if len(roots) != 2 || roots[0].Fn != "handle" || roots[1].Fn != "orphan" {
		t.Fatalf("roots: %+v", roots)
	}

	if got := roots[0].Title(regexp.MustCompile(`^(helper|Get)$`)).Fn; got != "Get" {
		t.Errorf("Title = %s, want the deepest match Get", got)
	}
	if got := roots[0].Title(regexp.MustCompile(`nothing`)).Fn; got != "handle" {
		t.Errorf("Title without a match = %s, want the root", got)
	}

	var b strings.Builder
	arrows, err := render(&b, roots[0], Options{Collapse: regexp.MustCompile(`^internal/entity$`)})
	if err != nil {
		t.Fatal(err)
	}
	got := b.String()
	for _, want := range []string{
		"autonumber",
		"participant store as store",
		"loop 2 times\n        dataentry->>+store: Get\n        store-->>-dataentry: \n    end",
		"dataentry->>+audit: Record",
		"dataentry-)jobs: [go] run",
		"dataentry-->>-Caller: ",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in:\n%s", want, got)
		}
	}
	if strings.Contains(got, "entity") {
		t.Errorf("collapsed package drawn:\n%s", got)
	}
	// Caller->dataentry, store Get (once, folded), audit Record, jobs run.
	if arrows != 4 {
		t.Errorf("arrows = %d, want 4", arrows)
	}
}

func TestRenderValues(t *testing.T) {
	roots, err := Read(strings.NewReader(trace))
	if err != nil {
		t.Fatal(err)
	}
	var b strings.Builder
	arrows, err := render(&b, roots[0], Options{Values: true})
	if err != nil {
		t.Fatal(err)
	}
	got := b.String()
	for _, want := range []string{
		// Differing values still fold, shown as "…".
		"loop 2 times\n        dataentry->>+store: Get(…)\n        store-->>-dataentry: …\n    end",
		`entity->>+audit: Record(op="#60;update#62;#59;")`,
		"audit-->>-entity: err: disk full",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in:\n%s", want, got)
		}
	}
	if arrows != 5 {
		t.Errorf("arrows = %d, want 5", arrows)
	}

	// Identical values keep them, and a trailing nil error is dropped.
	roots[0].Children[0].Children[1].Args = roots[0].Children[0].Children[0].Args
	roots[0].Children[0].Children[1].Results = roots[0].Children[0].Children[0].Results
	b.Reset()
	if _, err := render(&b, roots[0], Options{Values: true}); err != nil {
		t.Fatal(err)
	}
	if want := `store: Get(id="T-1")` + "\n        store-->>-dataentry: *entity.Entity{ID:T-1}\n"; !strings.Contains(b.String(), want) {
		t.Errorf("missing %q in:\n%s", want, b.String())
	}
}

func TestLabelOneLine(t *testing.T) {
	if got := label("a\nb\r`c"); got != "a b #96;c" {
		t.Errorf("label = %q", got)
	}
}

func TestCapLabel(t *testing.T) {
	long := strings.Repeat("é", maxLabel)
	got := capLabel(long)
	if !strings.HasSuffix(got, "…") || len(got) > maxLabel+len("…") || !utf8.ValidString(got) {
		t.Errorf("capLabel = %q", got)
	}
	if capLabel("short") != "short" {
		t.Error("short label changed")
	}
}

// render draws root and returns the number of arrows.
func render(w *strings.Builder, root *Call, opt Options) (int, error) {
	d := Build(root, opt)
	return d.Arrows(), d.Mermaid(w)
}
