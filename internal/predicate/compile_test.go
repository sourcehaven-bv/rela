package predicate_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/predicate"
)

// testEnv builds the env used across the accept/reject corpora. It
// includes one variable per declared shape and one function per
// expected host primitive. Tests share this env so any drift between
// the package's public surface and its expected use is caught here.
func testEnv(t *testing.T) *predicate.Env {
	t.Helper()
	env := predicate.NewEnv()
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatalf("env declare: %v", err)
		}
	}
	must(env.DeclareVar("entity", predicate.RecordType{
		"status":           predicate.StringType,
		"wont_fix_reason":  predicate.StringType,
		"created_by":       predicate.StringType,
		"assignee":         predicate.StringType,
		"effort":           predicate.StringType,
		"priority":         predicate.StringType,
		"title":            predicate.StringType,
		"count":            predicate.NumberType,
		"ratio":            predicate.NumberType,
		"frozen_for_audit": predicate.BoolType,
	}))
	must(env.DeclareVar("current_user", predicate.RecordType{
		"id":        predicate.StringType,
		"mfa_fresh": predicate.BoolType,
	}))
	must(env.DeclareVar("env", predicate.RecordType{
		"frozen_for_audit": predicate.BoolType,
		"read_only":        predicate.BoolType,
	}))
	// has_role takes the principal explicitly so the rule's subject
	// is visible — same discipline as has_relation / count_relations.
	must(env.DeclareFunc("has_role", predicate.FuncSig{
		Params: []predicate.Type{predicate.RecordType{}, predicate.StringType},
		Return: predicate.BoolType,
	}))
	// Relation-touching host fns take the subject entity as their
	// first arg explicitly — see crit round on TKT-2QI1: every rule
	// is supposed to make its subject visible.
	must(env.DeclareFunc("has_relation", predicate.FuncSig{
		Params: []predicate.Type{predicate.RecordType{}, predicate.StringType, predicate.RecordType{}},
		Return: predicate.BoolType,
	}))
	must(env.DeclareFunc("count_relations", predicate.FuncSig{
		Params: []predicate.Type{predicate.RecordType{}, predicate.StringType},
		Return: predicate.NumberType,
	}))
	must(env.DeclareFunc("is_one_of", predicate.FuncSig{
		Params:   []predicate.Type{predicate.AnyType},
		Variadic: predicate.AnyType,
		Return:   predicate.BoolType,
	}))
	return env
}

func TestCompile_AcceptsValidExpressions(t *testing.T) {
	env := testEnv(t)
	dir := filepath.Join("testdata", "accept")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read accept dir: %v", err)
	}
	if len(entries) < 15 {
		t.Fatalf("accept corpus must have >=15 files (AC1), found %d", len(entries))
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".lua") {
			continue
		}
		t.Run(e.Name(), func(t *testing.T) {
			path := filepath.Join(dir, e.Name())
			src, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read %s: %v", path, err)
			}
			if _, err := predicate.Compile(env, string(src)); err != nil {
				t.Fatalf("Compile failed for %s: %v\nsource:\n%s", e.Name(), err, src)
			}
		})
	}
}

// TestCompile_RejectsDisallowedConstructs runs each .lua file in
// testdata/reject/ through Compile and asserts:
//
//  1. Compile returns a non-nil error.
//  2. The error is the type named in the parallel .want file
//     (ParseError or CompileError).
//  3. The error message contains the substring on the .want file's
//     first line.
//
// The .want files double as user-facing-UX snapshots: each one says
// "when a rule author writes this mistake, they see this message."
// A drift in wording fails this test loudly. Update the .want file
// deliberately when changing a message — that's the workflow.
func TestCompile_RejectsDisallowedConstructs(t *testing.T) {
	env := testEnv(t)
	dir := filepath.Join("testdata", "reject")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read reject dir: %v", err)
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".lua") {
			continue
		}
		t.Run(e.Name(), func(t *testing.T) {
			path := filepath.Join(dir, e.Name())
			src, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read %s: %v", path, err)
			}
			wantPath := strings.TrimSuffix(path, ".lua") + ".want"
			wantRaw, err := os.ReadFile(wantPath)
			if err != nil {
				t.Fatalf("read %s: missing .want file (every reject case must declare expected error type + substring)", wantPath)
			}
			wantType, wantMsg, ok := parseWantSpec(string(wantRaw))
			if !ok {
				t.Fatalf("%s: malformed .want file (need: <type>\\t<substring>)", wantPath)
			}

			_, err = predicate.Compile(env, string(src))
			if err == nil {
				t.Fatalf("Compile unexpectedly accepted %s\nsource:\n%s", e.Name(), src)
			}

			switch wantType {
			case "ParseError":
				var pe *predicate.ParseError
				if !errors.As(err, &pe) {
					t.Fatalf("expected *ParseError, got %T: %v", err, err)
				}
			case "CompileError":
				var ce *predicate.CompileError
				if !errors.As(err, &ce) {
					t.Fatalf("expected *CompileError, got %T: %v", err, err)
				}
			default:
				t.Fatalf("%s: unknown want-type %q (use ParseError or CompileError)", wantPath, wantType)
			}

			if !strings.Contains(err.Error(), wantMsg) {
				t.Fatalf("error message does not match .want\n  got:  %s\n  want substring: %s", err.Error(), wantMsg)
			}
		})
	}
}

func TestCompile_RejectsUnknownSymbols(t *testing.T) {
	env := testEnv(t)
	cases := []struct {
		name string
		src  string
		want string
	}{
		{"unknown var", "missing_var == 'x'", "unknown identifier"},
		{"unknown attr", "entity.no_such_field == 'x'", "unknown attribute"},
		{"unknown func", "no_such_func()", "unknown function"},
		{"func used as var", "has_role == 'x'", "must be called"},
		{"var used as func", "entity()", "is a variable, not a function"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := predicate.Compile(env, tc.src)
			if err == nil {
				t.Fatalf("Compile unexpectedly accepted %q", tc.src)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error %q does not contain %q", err, tc.want)
			}
		})
	}
}

func TestCompile_RejectsNilEnv(t *testing.T) {
	_, err := predicate.Compile(nil, "true")
	if err == nil {
		t.Fatal("expected CompileError for nil env, got nil")
	}
	var ce *predicate.CompileError
	if !errors.As(err, &ce) {
		t.Fatalf("expected *CompileError, got %T", err)
	}
	if !strings.Contains(ce.Reason, "env must be non-nil") {
		t.Fatalf("unexpected reason: %s", ce.Reason)
	}
}

func TestCompile_RejectsDeeplyNestedExpression(t *testing.T) {
	env := testEnv(t)
	// gopher-lua's parser collapses paren-only nesting, so a paren bomb
	// doesn't reach the walker. We use a chained `and` instead, which
	// gopher-lua faithfully represents as nested LogicalOpExpr nodes.
	const depth = 1024
	var b strings.Builder
	b.WriteString("true")
	for range depth {
		b.WriteString(" and true")
	}
	_, err := predicate.Compile(env, b.String())
	if err == nil {
		t.Fatal("expected CompileError on deep nesting, got nil")
	}
	var ce *predicate.CompileError
	if !errors.As(err, &ce) {
		t.Fatalf("expected *CompileError, got %T: %v", err, err)
	}
	if !strings.Contains(ce.Reason, "nests too deeply") {
		t.Fatalf("unexpected reason: %s", ce.Reason)
	}
}

func TestCompile_NumberLexicalForms(t *testing.T) {
	env := testEnv(t)
	cases := []struct {
		src string
	}{
		{"entity.count == 1"},
		{"entity.count == 1.0"},
		{"entity.count == 1e10"},
		{"entity.count == 0xFF"},
		{"entity.count == 1.5e-3"},
	}
	for _, tc := range cases {
		t.Run(tc.src, func(t *testing.T) {
			if _, err := predicate.Compile(env, tc.src); err != nil {
				t.Fatalf("Compile %q: %v", tc.src, err)
			}
		})
	}
}

func TestCompile_StripsBOM(t *testing.T) {
	env := testEnv(t)
	const bom = "\xef\xbb\xbf"
	src := bom + "true"
	if _, err := predicate.Compile(env, src); err != nil {
		t.Fatalf("Compile %q: %v", src, err)
	}
}

func TestCompile_RejectsLeadingReturn(t *testing.T) {
	env := testEnv(t)
	cases := []string{
		"return false",
		"  return false",
		"-- a comment\nreturn false",
		"\xef\xbb\xbfreturn false",
	}
	for _, src := range cases {
		t.Run(src, func(t *testing.T) {
			_, err := predicate.Compile(env, src)
			if err == nil {
				t.Fatalf("expected reject for %q, got nil", src)
			}
		})
	}
}

// Confirm `returns` (an identifier containing `return` as a prefix) is
// NOT misclassified as a leading-return statement.
func TestCompile_LeadingReturnTokenBoundary(t *testing.T) {
	env := predicate.NewEnv()
	if err := env.DeclareVar("returns", predicate.BoolType); err != nil {
		t.Fatalf("declare: %v", err)
	}
	if _, err := predicate.Compile(env, "returns"); err != nil {
		t.Fatalf("Compile %q: %v", "returns", err)
	}
}

// TestCompile_ErrorLineNumbers verifies that multi-line sources
// produce errors that point at the line carrying the offending
// construct, not always line 1. Line numbers come from the
// gopher-lua AST; the test pins the discipline so a future change
// can't silently regress to "everything is line 1".
func TestCompile_ErrorLineNumbers(t *testing.T) {
	env := testEnv(t)
	cases := []struct {
		name     string
		src      string
		wantLine int
	}{
		{
			name:     "arithmetic on line 3",
			src:      "entity.status == 'x'\n  and entity.priority == 'y'\n  and entity.count + 1 < 10",
			wantLine: 3,
		},
		{
			name:     "unknown identifier on line 2",
			src:      "entity.status == 'x'\n  and bogus_var == 5",
			wantLine: 2,
		},
		{
			name:     "function literal on line 4",
			src:      "entity.status == 'a'\n  or entity.status == 'b'\n  or entity.status == 'c'\n  or (function() return true end)()",
			wantLine: 4,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := predicate.Compile(env, tc.src)
			if err == nil {
				t.Fatalf("expected error, got nil")
			}
			var ce *predicate.CompileError
			if !errors.As(err, &ce) {
				t.Fatalf("expected *CompileError, got %T: %v", err, err)
			}
			if ce.Line != tc.wantLine {
				t.Fatalf("CompileError.Line = %d, want %d (message: %s)", ce.Line, tc.wantLine, err)
			}
		})
	}
}

// TestParseError_LineColPositioning verifies that the gopher-lua
// parser-error translator extracts line and column correctly and
// adjusts for the synthetic "return " prefix. Uses inputs that are
// genuinely malformed Lua (not just non-expressions), so the
// describe-as-non-expression second-try parse also fails and
// ParseError is the final verdict.
func TestParseError_LineColPositioning(t *testing.T) {
	env := testEnv(t)
	cases := []struct {
		name     string
		src      string
		wantLine int
		// We do not assert col exactly because gopher-lua's column
		// numbers are 1-based and our shift heuristic is best-effort.
		// We assert col > 0 to confirm the parser delivered some
		// column information.
	}{
		{name: "stray punctuation on line 1", src: "@@@", wantLine: 1},
		{name: "stray punctuation on line 2", src: "entity.status\n  @@@", wantLine: 2},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := predicate.Compile(env, tc.src)
			if err == nil {
				t.Fatalf("expected error, got nil")
			}
			var pe *predicate.ParseError
			if !errors.As(err, &pe) {
				t.Fatalf("expected *ParseError, got %T: %v", err, err)
			}
			if pe.Line != tc.wantLine {
				t.Fatalf("ParseError.Line = %d, want %d (message: %s)", pe.Line, tc.wantLine, err)
			}
			if pe.Col <= 0 {
				t.Fatalf("ParseError.Col = %d, want > 0 (message: %s)", pe.Col, err)
			}
		})
	}
}

// parseWantSpec reads a .want file's first line as
// "<type>\t<substring>". Subsequent lines are ignored (room for
// commentary).
func parseWantSpec(raw string) (errType, wantSubstring string, ok bool) {
	first, _, _ := strings.Cut(raw, "\n")
	first = strings.TrimRight(first, "\r")
	t, msg, found := strings.Cut(first, "\t")
	if !found || t == "" || msg == "" {
		return "", "", false
	}
	return t, msg, true
}
