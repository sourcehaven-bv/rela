package script

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/lua"
)

// Simulates exactly what internal/scheduler/scheduler.go does:
// set deps.Capabilities, then call ExecuteFile.
func TestSchedulerStyleDepsCapsDropped(t *testing.T) {
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, "scripts"), 0o755)
	os.WriteFile(filepath.Join(root, "scripts", "s.lua"), []byte(`
if http == nil then error("HTTP CAPABILITY WAS DROPPED") end
print("http present")
`), 0o644)

	e := NewEngine()
	deps := lua.WriteDeps{ReadDeps: lua.ReadDeps{ProjectRoot: root, Capabilities: lua.Capabilities{HTTP: true}}}
	err := e.ExecuteFile(context.Background(), "s.lua", deps, nil, nil)
	if err != nil {
		t.Fatalf("deps.Capabilities did NOT reach the runtime: %v", err)
	}
}
