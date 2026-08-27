package sqlitestore_test

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/store/sqlitestore"
)

// TestSecondOpenIsRefused is the single-writer guarantee (DEC-LFSYNY).
//
// It matters more than a "nice error message" test: rela enforces `unique:`
// with an untransacted scan in entitymanager, so two processes over one
// database have NO uniqueness backstop and the violation is silent. Refusing
// the second opener is what makes the inherited single-process assumption
// correct by construction rather than by hope.
func TestSecondOpenIsRefused(t *testing.T) {
	path := filepath.Join(t.TempDir(), "locked.db")

	first, err := sqlitestore.Open(sqlitestore.Options{Path: path})
	if err != nil {
		t.Fatalf("first open: %v", err)
	}
	defer first.Close()

	_, err = sqlitestore.Open(sqlitestore.Options{Path: path})
	if err == nil {
		t.Fatal("second open succeeded; the store must refuse a concurrent opener")
	}

	// The error has to be actionable: an operator seeing it needs to know which
	// process to stop, so it carries the holder's pid.
	msg := err.Error()
	if !strings.Contains(msg, "another process") {
		t.Errorf("error does not explain the cause: %v", err)
	}
	if !strings.Contains(msg, "pid "+strconv.Itoa(os.Getpid())) {
		t.Errorf("error does not name the holding pid: %v", err)
	}
}

// TestLockReleasedOnClose covers the other half: refusing forever would be a
// worse bug than not locking at all, since a clean restart would be impossible.
func TestLockReleasedOnClose(t *testing.T) {
	path := filepath.Join(t.TempDir(), "reopen.db")

	first, err := sqlitestore.Open(sqlitestore.Options{Path: path})
	if err != nil {
		t.Fatalf("first open: %v", err)
	}
	if err := first.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	second, reopenErr := sqlitestore.Open(sqlitestore.Options{Path: path})
	if reopenErr != nil {
		t.Fatalf("reopen after close was refused: %v", reopenErr)
	}
	_ = second.Close()
}

// TestWALIsEnabled pins the network-filesystem guard. On local storage WAL is
// available, so a failure here means either the guard regressed or the test is
// running somewhere SQLite is genuinely unsafe — both worth failing on.
func TestWALIsEnabled(t *testing.T) {
	s, err := sqlitestore.Open(sqlitestore.Options{
		Path: filepath.Join(t.TempDir(), "wal.db"),
	})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer s.Close()

	if got := s.JournalMode(); got != "wal" {
		t.Errorf("journal_mode = %q, want %q", got, "wal")
	}
}
