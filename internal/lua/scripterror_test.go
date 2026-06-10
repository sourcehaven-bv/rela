package lua

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"testing/fstest"

	glua "github.com/yuin/gopher-lua"
)

// loadAndRun emulates how Runtime.RunFile loads scripts and, importantly,
// installs the same message handler so the test exercises the typed-frame
// path that BuildScriptError consumes in production. Returns the captured
// frames and the error.
func loadAndRun(t *testing.T, chunkname, code string) ([]StackFrame, error) {
	t.Helper()
	L := glua.NewState()
	t.Cleanup(L.Close)
	fn, err := L.Load(strings.NewReader(code), chunkname)
	if err != nil {
		return nil, fmt.Errorf("cannot compile script: %w", err)
	}
	var frames []StackFrame
	handler := L.NewFunction(func(ls *glua.LState) int {
		frames = collectStackFrames(ls)
		ls.Push(ls.Get(1))
		return 1
	})
	L.Push(fn)
	return frames, L.PCall(0, glua.MultRet, handler)
}

func TestBuildScriptError_RuntimeNilIndex(t *testing.T) {
	t.Parallel()
	frames, err := loadAndRun(t, "actions/foo.lua", "local x = nil\nreturn x.bar")
	if err == nil {
		t.Fatal("expected error")
	}

	se := BuildScriptError(BuildInput{
		Surface: SurfaceAction,
		Path:    "actions/foo.lua",
		Err:     err,
		Frames:  frames,
	})

	if se.LuaLine != 2 {
		t.Errorf("LuaLine=%d, want 2", se.LuaLine)
	}
	if !strings.Contains(se.LuaMessage, "attempt to index") {
		t.Errorf("LuaMessage=%q, want contains 'attempt to index'", se.LuaMessage)
	}
	if len(se.Stack) == 0 {
		t.Error("Stack is empty, want at least one frame")
	}
	// LuaMessage should NOT include the "<chunk>:<line>: " prefix anymore.
	if strings.HasPrefix(se.LuaMessage, "actions/foo.lua") {
		t.Errorf("LuaMessage still has chunk prefix: %q", se.LuaMessage)
	}
}

func TestBuildScriptError_CompileError(t *testing.T) {
	t.Parallel()
	frames, err := loadAndRun(t, "actions/bad.lua", "local x is broken")
	if err == nil {
		t.Fatal("expected compile error")
	}

	se := BuildScriptError(BuildInput{
		Surface: SurfaceAction,
		Path:    "actions/bad.lua",
		Err:     err,
		Frames:  frames, // empty for compile errors; included for completeness
	})

	if se.LuaLine != 1 {
		t.Errorf("LuaLine=%d, want 1 (parsed from line:N(column:N))", se.LuaLine)
	}
	if !strings.Contains(se.LuaMessage, "parse error") {
		t.Errorf("LuaMessage=%q, want contains 'parse error'", se.LuaMessage)
	}
	if len(se.Stack) != 0 {
		t.Errorf("Stack has %d frames; compile errors carry no stack", len(se.Stack))
	}
}

func TestBuildScriptError_ErrorRaisedWithTable(t *testing.T) {
	t.Parallel()
	frames, err := loadAndRun(t, "scripts/x.lua", "error({code=42})")
	if err == nil {
		t.Fatal("expected error")
	}

	se := BuildScriptError(BuildInput{
		Surface: SurfaceLuaRun,
		Path:    "scripts/x.lua",
		Err:     err,
		Frames:  frames,
	})

	// Non-string error: Object stringifies to "table: 0x...". The line
	// comes from the typed frame capture, not from any string parsing.
	if se.LuaLine != 1 {
		t.Errorf("LuaLine=%d, want 1 from typed frames", se.LuaLine)
	}
	if !strings.Contains(se.LuaMessage, "table:") {
		t.Errorf("LuaMessage=%q, want contains 'table:'", se.LuaMessage)
	}
}

func TestBuildScriptError_PlainGoError(t *testing.T) {
	t.Parallel()
	se := BuildScriptError(BuildInput{
		Surface: SurfaceAction,
		Path:    "actions/foo.lua",
		Err:     errors.New("script not found"),
	})

	if se.LuaMessage != "script not found" {
		t.Errorf("LuaMessage=%q", se.LuaMessage)
	}
	if se.LuaLine != 0 {
		t.Errorf("LuaLine=%d, want 0 for non-Lua error", se.LuaLine)
	}
	if len(se.Stack) != 0 {
		t.Error("Stack should be empty for non-Lua error")
	}
}

func TestBuildScriptError_PopulatesSource(t *testing.T) {
	t.Parallel()
	// Use `local q = z.foo` instead of `return z.foo` — a `return` followed
	// by another statement is a Lua syntax error, not a runtime one.
	const code = "local x = 1\nlocal y = 2\nlocal z = nil\nlocal q = z.foo\nlocal w = 3\n"
	frames, err := loadAndRun(t, "actions/foo.lua", code)
	if err == nil {
		t.Fatal("expected error")
	}

	srcFS := fstest.MapFS{
		"actions/foo.lua": &fstest.MapFile{Data: []byte(code)},
	}

	se := BuildScriptError(BuildInput{
		Surface:  SurfaceAction,
		Path:     "actions/foo.lua",
		Err:      err,
		Frames:   frames,
		SourceFS: srcFS,
	})

	if se.LuaLine != 4 {
		t.Fatalf("LuaLine=%d, want 4", se.LuaLine)
	}
	// ±3 context: lines 1..5
	wantLines := []int{1, 2, 3, 4, 5}
	if len(se.Source) != len(wantLines) {
		t.Fatalf("got %d source lines, want %d", len(se.Source), len(wantLines))
	}
	for i, sl := range se.Source {
		if sl.N != wantLines[i] {
			t.Errorf("source[%d].N=%d, want %d", i, sl.N, wantLines[i])
		}
		if (sl.N == 4) != sl.Highlight {
			t.Errorf("source[%d] highlight=%v, want %v", i, sl.Highlight, sl.N == 4)
		}
	}
}

func TestBuildScriptError_SourceClippedAtBoundaries(t *testing.T) {
	t.Parallel()
	const code = "local q = nil.foo\n"
	frames, err := loadAndRun(t, "actions/edge.lua", code)
	if err == nil {
		t.Fatal("expected error")
	}

	srcFS := fstest.MapFS{
		"actions/edge.lua": &fstest.MapFile{Data: []byte(code)},
	}
	se := BuildScriptError(BuildInput{
		Surface:  SurfaceAction,
		Path:     "actions/edge.lua",
		Err:      err,
		Frames:   frames,
		SourceFS: srcFS,
	})
	if se.LuaLine != 1 {
		t.Fatalf("LuaLine=%d, want 1", se.LuaLine)
	}
	if len(se.Source) != 1 {
		t.Fatalf("got %d lines, want 1", len(se.Source))
	}
	if se.Source[0].N != 1 || !se.Source[0].Highlight {
		t.Errorf("source[0]=%+v", se.Source[0])
	}
}

func TestBuildScriptError_SourceSkippedForOversizedFile(t *testing.T) {
	t.Parallel()
	const code = "local q = nil.foo\n"
	frames, err := loadAndRun(t, "actions/big.lua", code)

	bigData := make([]byte, maxSourceFileSize+1)
	for i := range bigData {
		bigData[i] = 'a'
	}
	srcFS := fstest.MapFS{
		"actions/big.lua": &fstest.MapFile{Data: bigData},
	}

	se := BuildScriptError(BuildInput{
		Surface:  SurfaceAction,
		Path:     "actions/big.lua",
		Err:      err,
		Frames:   frames,
		SourceFS: srcFS,
	})
	if len(se.Source) != 0 {
		t.Errorf("Source has %d lines; want none for oversized file", len(se.Source))
	}
	if se.LuaLine == 0 {
		t.Error("LuaLine must still be set even when source is skipped")
	}
}

func TestBuildScriptError_SourceRejectsTraversal(t *testing.T) {
	t.Parallel()
	const code = "local q = nil.foo\n"
	frames, err := loadAndRun(t, "../etc/passwd", code)

	// FS contains a real file at the would-be-traversed path, but our
	// path cleaner should refuse to descend into it.
	srcFS := fstest.MapFS{
		"../etc/passwd": &fstest.MapFile{Data: []byte("root:x:0:0::/root:/bin/sh\n")},
	}

	se := BuildScriptError(BuildInput{
		Surface:  SurfaceAction,
		Path:     "../etc/passwd",
		Err:      err,
		Frames:   frames,
		SourceFS: srcFS,
	})
	if len(se.Source) != 0 {
		t.Errorf("Source populated for traversal-y path: %+v", se.Source)
	}
}

func TestBuildScriptError_RedactsArgsByKey(t *testing.T) {
	t.Parallel()
	se := BuildScriptError(BuildInput{
		Surface: SurfaceAction,
		Path:    "actions/x.lua",
		Args: map[string]any{
			"username":      "alice",
			"password":      "hunter2",
			"api_key":       "anything",
			"api-key":       "also-redacted",
			"AUTHORIZATION": "Bearer xxx",
			"client_secret": "shh",
		},
		Err: errors.New("any error"),
	})

	cases := []struct {
		key     string
		want    string
		isExact bool
	}{
		{"username", "alice", true},
		{"password", redactedPlaceholder, true},
		{"api_key", redactedPlaceholder, true},
		{"api-key", redactedPlaceholder, true},
		{"AUTHORIZATION", redactedPlaceholder, true},
		{"client_secret", redactedPlaceholder, true},
	}
	for _, tc := range cases {
		got, ok := se.Args[tc.key]
		if !ok {
			t.Errorf("missing key %q in redacted args", tc.key)
			continue
		}
		if got != tc.want {
			t.Errorf("args[%q]=%v, want %v", tc.key, got, tc.want)
		}
	}
}

func TestBuildScriptError_RedactsValueShape(t *testing.T) {
	t.Parallel()
	jwt := "eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiJ4In0.signature"
	longHex := strings.Repeat("a1b2c3d4", 8) // 64 hex chars
	longB64 := strings.Repeat("AbCdEfGh", 5) // 40 b64-ish chars
	short := "hello"
	// "abc " contains a space so it doesn't match any value-shape regex;
	// repeated, it exceeds the per-string length cap and gets truncated.
	huge := strings.Repeat("abc ", (maxStringValueLen/4)+10)

	se := BuildScriptError(BuildInput{
		Surface: SurfaceAction,
		Path:    "x",
		Args: map[string]any{
			"jwt":      jwt,
			"long_hex": longHex,
			"long_b64": longB64,
			"short":    short,
			"huge":     huge,
		},
		Err: errors.New("e"),
	})

	if got := se.Args["jwt"]; got != redactedPlaceholder {
		t.Errorf("jwt=%v, want redacted", got)
	}
	if got := se.Args["long_hex"]; got != redactedPlaceholder {
		t.Errorf("long_hex=%v, want redacted", got)
	}
	if got := se.Args["long_b64"]; got != redactedPlaceholder {
		t.Errorf("long_b64=%v, want redacted", got)
	}
	if got := se.Args["short"]; got != short {
		t.Errorf("short=%v, want %v", got, short)
	}
	gotHuge := se.Args["huge"].(string)
	if !strings.HasSuffix(gotHuge, truncatedPlaceholder) {
		t.Errorf("huge missing truncation marker: %q", gotHuge[len(gotHuge)-30:])
	}
	if len(gotHuge) > maxStringValueLen+len(truncatedPlaceholder) {
		t.Errorf("huge len=%d, want <= %d", len(gotHuge), maxStringValueLen+len(truncatedPlaceholder))
	}
}

func TestBuildScriptError_RedactsNestedArgs(t *testing.T) {
	t.Parallel()
	se := BuildScriptError(BuildInput{
		Surface: SurfaceAction,
		Path:    "x",
		Args: map[string]any{
			"outer": map[string]any{
				"password": "leak1",
				"inner": []any{
					map[string]any{"token": "leak2"},
				},
			},
		},
		Err: errors.New("e"),
	})

	outer := se.Args["outer"].(map[string]any)
	if outer["password"] != redactedPlaceholder {
		t.Errorf("nested password=%v", outer["password"])
	}
	innerSlice := outer["inner"].([]any)
	innerMap := innerSlice[0].(map[string]any)
	if innerMap["token"] != redactedPlaceholder {
		t.Errorf("doubly-nested token=%v", innerMap["token"])
	}
}

func TestBuildScriptError_CapturedOutputRedactedAndCapped(t *testing.T) {
	t.Parallel()
	jwt := "eyJabc.def.ghi"
	captured := []byte("hello\n" + jwt + "\nworld\n")

	se := BuildScriptError(BuildInput{
		Surface:        SurfaceAction,
		Path:           "x",
		Err:            errors.New("e"),
		CapturedOutput: captured,
	})

	if !strings.Contains(se.CapturedOutput, "hello") {
		t.Errorf("missing 'hello' in %q", se.CapturedOutput)
	}
	if strings.Contains(se.CapturedOutput, jwt) {
		t.Errorf("JWT not redacted in captured output: %q", se.CapturedOutput)
	}
	if !strings.Contains(se.CapturedOutput, redactedPlaceholder) {
		t.Errorf("captured output missing redaction marker: %q", se.CapturedOutput)
	}
}

func TestBuildScriptError_CapturedOutputCappedAt16K(t *testing.T) {
	t.Parallel()
	// Build 32KB of distinct lines so the value-shape redactor doesn't
	// collapse the whole thing to "<redacted>" before truncation kicks in.
	var sb strings.Builder
	for i := 0; sb.Len() < maxCapturedOutputBytes*2; i++ {
		fmt.Fprintf(&sb, "line %d says hello world\n", i)
	}
	huge := []byte(sb.String())

	se := BuildScriptError(BuildInput{
		Surface:        SurfaceAction,
		Path:           "x",
		Err:            errors.New("e"),
		CapturedOutput: huge,
	})
	if !strings.HasSuffix(se.CapturedOutput, " more bytes]") {
		t.Errorf("captured output missing truncation suffix; tail: %q", tail(se.CapturedOutput, 60))
	}
	if len(se.CapturedOutput) > maxCapturedOutputBytes+64 {
		t.Errorf("captured output too long: %d", len(se.CapturedOutput))
	}
}

func TestScriptError_Error(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		se   *ScriptError
		want string
	}{
		{
			name: "with path and line",
			se:   &ScriptError{Path: "a/b.lua", LuaLine: 12, LuaMessage: "boom"},
			want: "a/b.lua:12: boom",
		},
		{
			name: "with path no line",
			se:   &ScriptError{Path: "a/b.lua", LuaMessage: "boom"},
			want: "a/b.lua: boom",
		},
		{
			name: "no path",
			se:   &ScriptError{LuaMessage: "boom"},
			want: "boom",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.se.Error(); got != tc.want {
				t.Errorf("Error()=%q, want %q", got, tc.want)
			}
		})
	}
}

func tail(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[len(s)-n:]
}
