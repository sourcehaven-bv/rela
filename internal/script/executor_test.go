package script

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/entitymanager/entitymanagertest"
	"github.com/Sourcehaven-BV/rela/internal/lua"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
	"github.com/Sourcehaven-BV/rela/internal/tracer"
)

func testWriteDeps(projectRoot string) lua.WriteDeps {
	st := memstore.New()
	return lua.WriteDeps{
		ReadDeps: lua.ReadDeps{
			Store:       st,
			Tracer:      tracer.New(st),
			ProjectRoot: projectRoot,
		},
		EntityManager: entitymanagertest.PanicOnUse{},
	}
}

func TestEngine_ExecuteFile_PathTraversal(t *testing.T) {
	engine := NewEngine()
	err := engine.ExecuteFile(context.Background(), "../../../etc/passwd", testWriteDeps("/project"), nil, nil)
	if err == nil {
		t.Fatal("expected error for path traversal, got none")
	}
	if !strings.Contains(err.Error(), "local") {
		t.Errorf("expected path traversal error, got: %v", err)
	}
}

func TestEngine_ExecuteFile_AbsolutePath(t *testing.T) {
	engine := NewEngine()
	err := engine.ExecuteFile(context.Background(), "/etc/passwd", testWriteDeps("/project"), nil, nil)
	if err == nil {
		t.Fatal("expected error for absolute path, got none")
	}
	if !strings.Contains(err.Error(), "local") {
		t.Errorf("expected path error, got: %v", err)
	}
}

func TestEngine_ExecuteFile_WrongExtension(t *testing.T) {
	engine := NewEngine()
	err := engine.ExecuteFile(context.Background(), "script.txt", testWriteDeps("/project"), nil, nil)
	if err == nil {
		t.Fatal("expected error for wrong extension, got none")
	}
	if !strings.Contains(err.Error(), ".lua") {
		t.Errorf("expected extension error, got: %v", err)
	}
}

func TestEngine_ExecuteFile_ValidPath(t *testing.T) {
	engine := NewEngine()
	err := engine.ExecuteFile(context.Background(), "test.lua", testWriteDeps("/nonexistent"), nil, nil)
	if err == nil {
		t.Fatal("expected error for missing project directory")
	}
	if strings.Contains(err.Error(), "local") || strings.Contains(err.Error(), ".lua") {
		t.Errorf("expected filesystem error, not validation error, got: %v", err)
	}
}

// writeDocScript creates scripts/<name> under tempRoot with the given body
// and returns the tempRoot. Used by ExecuteDocument tests.
func writeDocScript(t *testing.T, name, body string) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, scriptsDir), 0o755); err != nil {
		t.Fatalf("mkdir scripts: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, scriptsDir, name), []byte(body), 0o644); err != nil {
		t.Fatalf("write script: %v", err)
	}
	return root
}

// TestExecuteDocument_CapturesStdout verifies the core contract of
// ExecuteDocument: the script's print() output lands in the caller's
// writer, ready to be used as markdown for downstream HTML conversion.
func TestExecuteDocument_CapturesStdout(t *testing.T) {
	root := writeDocScript(t, "doc.lua", `print("# " .. rela.document.id)
print("entry: " .. rela.document.entry_id)
print("mode: " .. rela.mode)`)

	var stdout bytes.Buffer
	engine := NewEngine()
	err := engine.ExecuteDocument("doc.lua", testWriteDeps(root), &stdout,
		"release-notes", "REL-001", 0)
	if err != nil {
		t.Fatalf("ExecuteDocument failed: %v", err)
	}

	got := stdout.String()
	want := "# release-notes\nentry: REL-001\nmode: document\n"
	if got != want {
		t.Errorf("stdout mismatch:\n got: %q\nwant: %q", got, want)
	}
}

// TestExecuteDocument_TimeoutEnforced verifies that a non-zero timeout
// is honored for document-mode renders (AC11). An infinite loop with a
// 1-second budget must terminate well under 2 seconds.
func TestExecuteDocument_TimeoutEnforced(t *testing.T) {
	root := writeDocScript(t, "spin.lua", `while true do end`)

	var stdout bytes.Buffer
	engine := NewEngine()
	start := time.Now()
	err := engine.ExecuteDocument("spin.lua", testWriteDeps(root), &stdout,
		"id", "entry", 1*time.Second)
	elapsed := time.Since(start)
	if err == nil {
		t.Fatal("expected timeout error, got none")
	}
	if elapsed > 2500*time.Millisecond {
		t.Errorf("timeout took %v, expected < 2.5s", elapsed)
	}
}

// TestExecuteDocument_BadPath surfaces the same path-validation errors as
// ExecuteFile — ExecuteDocument reuses loadScript so the existing
// traversal / extension / local-path checks apply.
func TestExecuteDocument_BadPath(t *testing.T) {
	var stdout bytes.Buffer
	engine := NewEngine()
	err := engine.ExecuteDocument("../../etc/passwd", testWriteDeps("/project"),
		&stdout, "id", "entry", 0)
	if err == nil {
		t.Fatal("expected error for path traversal, got none")
	}
	if !strings.Contains(err.Error(), "local") {
		t.Errorf("expected path traversal error, got: %v", err)
	}
}
