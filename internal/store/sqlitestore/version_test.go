package sqlitestore_test

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/store/sqlitestore"
)

// TestSchemaVersionStamped pins that a fresh database records its shape.
// Without the stamp, a future migration cannot tell v1 from v2 and would have
// to guess from pragma_table_info.
func TestSchemaVersionStamped(t *testing.T) {
	path := filepath.Join(t.TempDir(), "v.db")
	s, err := sqlitestore.Open(sqlitestore.Options{Path: path})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	_ = s.Close()

	out, err := exec.Command("sqlite3", path, "PRAGMA user_version;").Output()
	if err != nil {
		t.Skipf("sqlite3 CLI unavailable: %v", err)
	}
	if got := strings.TrimSpace(string(out)); got != "1" {
		t.Errorf("user_version = %q, want %q", got, "1")
	}
}

// TestRefusesNewerSchema is the forward-only guarantee: a database written by
// a newer rela must be refused, not opened and silently mis-read.
func TestRefusesNewerSchema(t *testing.T) {
	path := filepath.Join(t.TempDir(), "future.db")
	s, err := sqlitestore.Open(sqlitestore.Options{Path: path})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	_ = s.Close()

	if err := exec.Command("sqlite3", path, "PRAGMA user_version = 99;").Run(); err != nil {
		t.Skipf("sqlite3 CLI unavailable: %v", err)
	}

	_, reopenErr := sqlitestore.Open(sqlitestore.Options{Path: path})
	if reopenErr == nil {
		t.Fatal("opened a database from a newer rela; must refuse")
	}
	if !strings.Contains(reopenErr.Error(), "newer rela") {
		t.Errorf("error does not explain the version mismatch: %v", reopenErr)
	}
}
