package logtest

import (
	"bytes"
	"log"
	"log/slog"
	"strings"
	"testing"
)

// TestCapture_RestoresLogOutput: after a capture ends, a logger built from
// the original slog.Default writes where it did before, not into the finished
// capture (BUG-FLON0Z).
func TestCapture_RestoresLogOutput(t *testing.T) {
	var sink bytes.Buffer
	prevOut, prevFlags := log.Writer(), log.Flags()
	t.Cleanup(func() {
		log.SetOutput(prevOut)
		log.SetFlags(prevFlags)
	})
	log.SetOutput(&sink)
	original := slog.Default()

	var captured func() string
	t.Run("capture", func(t *testing.T) {
		captured = Capture(t, slog.LevelInfo)
		slog.Info("during")
	})
	original.Info("after")

	if got := captured(); !strings.Contains(got, "during") || strings.Contains(got, "after") {
		t.Errorf("capture = %q, want only the line logged during the capture", got)
	}
	if !strings.Contains(sink.String(), "after") {
		t.Errorf("log output = %q, want the line logged after the capture", sink.String())
	}
	if log.Flags() != prevFlags {
		t.Errorf("log flags = %d, want %d", log.Flags(), prevFlags)
	}
}
