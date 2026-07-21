package docs

import (
	"errors"
	"testing"
)

func TestParse_LiteralOnly(t *testing.T) {
	t.Parallel()
	segs, err := parse("# Title\n\nSome prose.\n")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(segs) != 1 || segs[0].kind != segLiteral {
		t.Fatalf("want one literal segment, got %+v", segs)
	}
	if segs[0].body != "# Title\n\nSome prose.\n" {
		t.Errorf("literal body = %q", segs[0].body)
	}
}

func TestParse_StatementIsland(t *testing.T) {
	t.Parallel()
	src := "before\n```rela\nh2(\"X\")\nmd(\"y\")\n```\nafter\n"
	segs, err := parse(src)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	// literal "before", statement island, literal "after".
	if len(segs) != 3 {
		t.Fatalf("want 3 segments, got %d: %+v", len(segs), segs)
	}
	if segs[1].kind != segStatement {
		t.Fatalf("segment 1 should be statement, got %v", segs[1].kind)
	}
	if segs[1].body != "h2(\"X\")\nmd(\"y\")\n" {
		t.Errorf("island body = %q", segs[1].body)
	}
	// The island body begins on manual line 3 (```rela is line 2).
	if segs[1].line != 3 {
		t.Errorf("island line = %d, want 3", segs[1].line)
	}
}

func TestParse_UnterminatedFence(t *testing.T) {
	t.Parallel()
	_, err := parse("```rela\nh1(\"x\")\n")
	var be *BuildError
	if !errors.As(err, &be) {
		t.Fatalf("want *BuildError, got %T: %v", err, err)
	}
	if be.Kind != "parse" {
		t.Errorf("kind = %q, want parse", be.Kind)
	}
}

func TestParse_EchoSpan(t *testing.T) {
	t.Parallel()
	segs, err := parse("There are `rela count{type=\"x\"}` items.\n")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	// literal "There are ", echo, literal " items.\n"
	if len(segs) != 3 {
		t.Fatalf("want 3 segments, got %d: %+v", len(segs), segs)
	}
	if segs[1].kind != segEcho {
		t.Fatalf("middle segment should be echo, got %v", segs[1].kind)
	}
	if segs[1].body != `count{type="x"}` {
		t.Errorf("echo expr = %q", segs[1].body)
	}
}

func TestParse_RelaProseMentionNotEcho(t *testing.T) {
	t.Parallel()
	// Prose mentioning a `rela …` command (no call syntax) must stay literal.
	for _, src := range []string{
		"Run `rela docs build` to render.\n",
		"Use `rela init` first.\n",
	} {
		segs, err := parse(src)
		if err != nil {
			t.Fatalf("parse(%q): %v", src, err)
		}
		for _, s := range segs {
			if s.kind == segEcho {
				t.Errorf("prose command mention became an echo in %q: %+v", src, segs)
			}
		}
	}
	// But a real resolver call (has () or {}) IS an echo.
	segs, _ := parse("count is `rela count{type=\"x\"}` here\n")
	found := false
	for _, s := range segs {
		if s.kind == segEcho {
			found = true
		}
	}
	if !found {
		t.Errorf("resolver call should be an echo: %+v", segs)
	}
}

func TestParse_NonRelaCodeSpanUntouched(t *testing.T) {
	t.Parallel()
	// A plain code span without the `rela ` marker must pass through as literal.
	segs, err := parse("Use `go build` to compile.\n")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	for _, s := range segs {
		if s.kind == segEcho {
			t.Fatalf("plain code span must not become an echo: %+v", segs)
		}
	}
}

func TestParse_RelaInsideGenericFenceIsLiteral(t *testing.T) {
	t.Parallel()
	// A ```go (or any non-rela) fenced block that *shows* the doc language must
	// render literally — no island parsing inside.
	src := "```markdown\n```rela\nh1(\"nope\")\n```\n```\nreal text\n"
	segs, err := parse(src)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	for _, s := range segs {
		if s.kind == segStatement {
			t.Fatalf("no statement island should be parsed inside a generic fence: %+v", s)
		}
	}
}

func TestParse_EchoLineTracking(t *testing.T) {
	t.Parallel()
	src := "line1\nline2 `rela count{type=\"x\"}` end\nline3\n"
	segs, err := parse(src)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	var echo *segment
	for i := range segs {
		if segs[i].kind == segEcho {
			echo = &segs[i]
		}
	}
	if echo == nil {
		t.Fatal("expected an echo segment")
	}
	if echo.line != 2 {
		t.Errorf("echo line = %d, want 2", echo.line)
	}
}
