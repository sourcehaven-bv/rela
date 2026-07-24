package transform

import (
	"context"
	"errors"
	"os/exec"
	"runtime"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

func skipOnWindows(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("POSIX command fixtures not available on Windows")
	}
}

func TestRegistry_FromMarkdown(t *testing.T) {
	r := Registry{
		"pdf":   {From: FormatMarkdown, Command: []string{"pandoc"}, Produces: "application/pdf"},
		"docx":  {From: FormatMarkdown, Command: []string{"pandoc"}, Produces: "application/vnd.openxmlformats-officedocument.wordprocessingml.document"},
		"weird": {From: "html", Command: []string{"x"}, Produces: "text/html"},
	}
	got := r.FromMarkdown()
	if len(got) != 2 {
		t.Fatalf("want 2 markdown transforms, got %d", len(got))
	}
	// sorted by name
	if got[0].Name != "docx" || got[1].Name != "pdf" {
		t.Errorf("want [docx pdf], got [%s %s]", got[0].Name, got[1].Name)
	}
}

func TestEngine_Run_Identity(t *testing.T) {
	skipOnWindows(t)
	reg := Registry{
		"cat": {From: FormatMarkdown, Command: []string{"cat"}, Produces: "text/plain"},
	}
	res, err := NewEngine().Run(context.Background(), reg, "cat", RendererFunc(func(context.Context) ([]byte, error) {
		return []byte("# Hi\n"), nil
	}))
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if string(res.Data) != "# Hi\n" {
		t.Errorf("data = %q", res.Data)
	}
	if res.Produces != "text/plain" {
		t.Errorf("produces = %q", res.Produces)
	}
}

func TestEngine_Run_InOutFiles(t *testing.T) {
	skipOnWindows(t)
	if _, err := exec.LookPath("cp"); err != nil {
		t.Skip("cp not available")
	}
	reg := Registry{
		"cp": {From: FormatMarkdown, Command: []string{"cp", "{in}", "{out}"}, Produces: "application/octet-stream"},
	}
	res, err := NewEngine().Run(context.Background(), reg, "cp", RendererFunc(func(context.Context) ([]byte, error) {
		return []byte("payload"), nil
	}))
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if string(res.Data) != "payload" {
		t.Errorf("data = %q", res.Data)
	}
}

func TestEngine_Run_UnknownTransform(t *testing.T) {
	_, err := NewEngine().Run(context.Background(), Registry{}, "nope", RendererFunc(func(context.Context) ([]byte, error) {
		return nil, nil
	}))
	var unk UnknownTransformError
	if !errors.As(err, &unk) {
		t.Fatalf("want UnknownTransformError, got %v", err)
	}
	if unk.Name != "nope" {
		t.Errorf("name = %q", unk.Name)
	}
}

func TestEngine_Run_RenderError(t *testing.T) {
	skipOnWindows(t)
	reg := Registry{
		"cat": {From: FormatMarkdown, Command: []string{"cat"}, Produces: "text/plain"},
	}
	boom := errors.New("boom")
	_, err := NewEngine().Run(context.Background(), reg, "cat", RendererFunc(func(context.Context) ([]byte, error) {
		return nil, boom
	}))
	if !errors.Is(err, boom) {
		t.Fatalf("render error should propagate, got %v", err)
	}
}

func TestEngine_Probe_MissingBinary(t *testing.T) {
	probes := NewEngine().Probe(Registry{
		"ghost": {From: FormatMarkdown, Command: []string{"definitely-not-a-real-binary-xyz"}, Produces: "application/pdf"},
	})
	if probes["ghost"] == nil {
		t.Error("missing binary should produce a probe error")
	}
	if !strings.Contains(probes["ghost"].Error(), "not found on PATH") {
		t.Errorf("probe error should mention PATH, got %v", probes["ghost"])
	}
}

func TestEntityRenderer_Render(t *testing.T) {
	e := entity.New("TKT-1", "ticket")
	e.SetString("title", "Do the thing")
	e.SetString("status", "in-progress")
	e.SetString("priority", "high")
	e.Properties["tags"] = []string{"a", "b"}
	e.Content = "Body text here."

	er := EntityRenderer{
		Entity: e,
		Relations: []RelationGroup{
			{Label: "implements", Neighbors: []string{"Feature X", "Feature Y"}},
			{Label: "empty", Neighbors: nil}, // omitted
		},
	}
	out, err := er.Render(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	for _, want := range []string{
		"# Do the thing",
		"**priority:** high", // bold-label definition list, no header row
		"**tags:** a, b",
		"## implements",
		"- Feature X",
		"Body text here.",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("output missing %q\n---\n%s", want, s)
		}
	}
	if strings.Contains(s, "| Property | Value |") {
		t.Error("should be a definition list, not a Property|Value table")
	}
	if strings.Contains(s, "## empty") {
		t.Error("empty relation group should be omitted")
	}
	if strings.Contains(s, "**title:**") {
		t.Error("title should be the H1, not a property line")
	}
	if strings.Contains(s, "**status:**") {
		t.Error("status is workflow machinery, not document content")
	}
}

func TestEntityRenderer_EnumValueLabels(t *testing.T) {
	meta := &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"ticket": {
				Properties: map[string]metamodel.PropertyDef{
					"priority": {Type: "enum", Labels: map[string]string{"high": "🔥 High"}},
				},
				PropertyOrder: []string{"priority"},
			},
		},
	}
	e := entity.New("TKT-1", "ticket")
	e.SetString("priority", "high")
	er := EntityRenderer{Entity: e, Meta: meta}
	out, _ := er.Render(context.Background())
	if !strings.Contains(string(out), "**priority:** 🔥 High") {
		t.Errorf("enum value should use the metamodel display label:\n%s", out)
	}
}

func TestEntityRenderer_EscapesPipes(t *testing.T) {
	e := entity.New("X-1", "ticket")
	e.SetString("title", "a | b")
	e.SetString("note", "c | d\nsecond line")
	er := EntityRenderer{Entity: e}
	out, err := er.Render(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	if !strings.Contains(s, "# a \\| b") {
		t.Errorf("title pipe not escaped: %s", s)
	}
	// The note cell must have its newline collapsed to a space and its pipe escaped.
	if !strings.Contains(s, "c \\| d second line") {
		t.Errorf("cell not escaped/collapsed: %s", s)
	}
}

func TestEntityRenderer_ExtraPropertiesAreDeterministic(t *testing.T) {
	// Properties the metamodel does not order (or no metamodel at all) must render
	// in a stable, sorted sequence — not the Go map's randomized range order — so
	// exports are reproducible and diffable.
	e := entity.New("X-1", "thing")
	e.SetString("zeta", "z")
	e.SetString("alpha", "a")
	e.SetString("mu", "m")
	er := EntityRenderer{Entity: e}

	first, err := er.Render(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	s := string(first)
	ai, mi, zi := strings.Index(s, "**alpha:**"), strings.Index(s, "**mu:**"), strings.Index(s, "**zeta:**")
	if ai < 0 || ai >= mi || mi >= zi {
		t.Fatalf("extra props not in sorted order (alpha<mu<zeta): %s", s)
	}
	// Re-render several times: output must be byte-identical every run.
	for i := range 20 {
		out, err := er.Render(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		if string(out) != s {
			t.Fatalf("render %d differs from first — non-deterministic order:\n%s\n---\n%s", i, s, out)
		}
	}
}

func TestEntityRenderer_NilEntity(t *testing.T) {
	er := EntityRenderer{}
	if _, err := er.Render(context.Background()); err == nil {
		t.Error("nil entity should error")
	}
}
