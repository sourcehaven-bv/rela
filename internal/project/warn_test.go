package project

import (
	"os"
	"strings"
	"sync"
	"testing"
)

// captureStderr swaps os.Stderr for a pipe and returns what was written.
func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	orig := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stderr = w

	done := make(chan string, 1)
	go func() {
		var sb strings.Builder
		buf := make([]byte, 1024)
		for {
			n, readErr := r.Read(buf)
			sb.Write(buf[:n])
			if readErr != nil {
				break
			}
		}
		done <- sb.String()
	}()

	fn()
	w.Close()
	os.Stderr = orig
	return <-done
}

// resetWarnOnce lets each test observe the first-warning behavior; production
// deliberately keeps a process-lifetime Once.
func resetWarnOnce() { legacySchemaWarning = sync.Once{} }

// TestWarnIfLegacySchema covers the deprecation notice reachability fix
// (RR-9XXI80): every entry point calls this at startup, so it must warn for a
// legacy project, stay silent otherwise, and never repeat itself.
func TestWarnIfLegacySchema(t *testing.T) {
	t.Run("warns once for a legacy project", func(t *testing.T) {
		resetWarnOnce()
		ctx := &Context{SchemaIsLegacy: true}

		out := captureStderr(t, func() {
			WarnIfLegacySchema(ctx)
			// Servers open projects repeatedly; the notice must not accumulate.
			WarnIfLegacySchema(ctx)
			WarnIfLegacySchema(ctx)
		})

		if n := strings.Count(out, LegacySchemaFile); n != 1 {
			t.Errorf("warning appeared %d times, want exactly 1\ngot: %q", n, out)
		}
		// The notice is useless unless it says what to run.
		if !strings.Contains(out, "rela migrate") {
			t.Errorf("warning should name the fix command, got: %q", out)
		}
		if !strings.Contains(out, SchemaFile) {
			t.Errorf("warning should name the new file, got: %q", out)
		}
	})

	t.Run("silent for a current project", func(t *testing.T) {
		resetWarnOnce()
		out := captureStderr(t, func() {
			WarnIfLegacySchema(&Context{SchemaIsLegacy: false})
		})
		if out != "" {
			t.Errorf("expected no output, got %q", out)
		}
	})

	t.Run("nil context is a no-op", func(t *testing.T) {
		resetWarnOnce()
		out := captureStderr(t, func() { WarnIfLegacySchema(nil) })
		if out != "" {
			t.Errorf("expected no output, got %q", out)
		}
	})
}
