// Package logtest swaps the process-wide default logger in tests and puts
// all of it back afterwards.
//
// slog.SetDefault with a handler other than the original default also points
// the log package output at that handler. Setting the old default back does
// not undo that. So a test that only restores slog.Default leaves every
// logger built from the original slog.Default (a job queue's, for one)
// writing into its finished buffer (BUG-FLON0Z). The helpers here restore the
// log package output and flags as well.
//
// The package imports only the standard library, so any package's tests can
// use it. Tests that call it must not use t.Parallel: the default logger is
// process-wide.
package logtest

import (
	"bytes"
	"log"
	"log/slog"
	"sync"
	"testing"
)

// SetDefault makes slog.New(h) the default logger until the test ends.
func SetDefault(tb testing.TB, h slog.Handler) {
	tb.Helper()
	prevLogger, prevOut, prevFlags := slog.Default(), log.Writer(), log.Flags()
	tb.Cleanup(func() {
		slog.SetDefault(prevLogger)
		log.SetOutput(prevOut)
		log.SetFlags(prevFlags)
	})
	slog.SetDefault(slog.New(h))
}

// Capture sends the default logger's output at level and above to a buffer
// until the test ends, and returns a func that reads what was logged. Writes
// and reads are serialized, because goroutines the test started may log
// while it reads.
func Capture(tb testing.TB, level slog.Level) func() string {
	tb.Helper()
	w := &Buffer{}
	SetDefault(tb, slog.NewTextHandler(w, &slog.HandlerOptions{Level: level}))
	return w.String
}

// Buffer is a bytes.Buffer that is safe for concurrent writes and reads.
type Buffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *Buffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *Buffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}
