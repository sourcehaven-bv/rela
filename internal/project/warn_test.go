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

// resetWarnOnce clears the warned-roots set so each test starts fresh.
//
// This and captureStderr mutate package-global and process-global state
// (os.Stderr) respectively, so nothing in this file may use t.Parallel().
func resetWarnOnce() { warnedLegacyRoots = sync.Map{} }

// TestWarnIfLegacySchema covers the deprecation notice reachability fix
// (RR-9XXI80): every entry point calls this at startup, so it must warn for a
// legacy project, stay silent otherwise, and never repeat itself.
func TestWarnIfLegacySchema(t *testing.T) {
	t.Run("warns once per project", func(t *testing.T) {
		resetWarnOnce()
		ctx := &Context{Root: "/proj-a", SchemaIsLegacy: true}

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

	// A single process-wide sync.Once would warn for the first legacy project
	// and then stay silent for every later one — wrong for the desktop app,
	// which opens many projects in one long-lived process, and which would
	// leave the operator believing the later projects were fine (RR-K2ELC7).
	t.Run("warns again for a different legacy project", func(t *testing.T) {
		resetWarnOnce()

		out := captureStderr(t, func() {
			WarnIfLegacySchema(&Context{Root: "/proj-a", SchemaIsLegacy: true})
			WarnIfLegacySchema(&Context{Root: "/proj-b", SchemaIsLegacy: true})
		})

		if n := strings.Count(out, LegacySchemaFile); n != 2 {
			t.Errorf("warning appeared %d times for 2 distinct projects, want 2\ngot: %q", n, out)
		}
		// Naming the root is what makes two warnings distinguishable.
		if !strings.Contains(out, "/proj-a") || !strings.Contains(out, "/proj-b") {
			t.Errorf("each warning should name its project root, got: %q", out)
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
