package cli

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"
)

// version_test.go covers VersionCmd output. Cobra-era tests inspecting
// rootCmd.Commands() and rootCmd.Version are gone — kong handles the
// command tree via the CLI struct (covered by root_test.go).

// runVersionCmd executes VersionCmd while capturing os.Stdout.
func runVersionCmd(t *testing.T) string {
	t.Helper()
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	if err := (&VersionCmd{}).Run(); err != nil {
		t.Fatalf("VersionCmd.Run: %v", err)
	}
	w.Close()
	os.Stdout = old
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	return buf.String()
}

func TestVersionCommand(t *testing.T) {
	oldVersion := Version
	defer func() { Version = oldVersion }()

	Version = "1.2.3-test"
	got := runVersionCmd(t)

	expected := "rela version 1.2.3-test\n"
	if got != expected {
		t.Errorf("version output = %q, want %q", got, expected)
	}
}

func TestVersionCommandContainsVersion(t *testing.T) {
	oldVersion := Version
	defer func() { Version = oldVersion }()

	Version = "test-version-123"
	got := runVersionCmd(t)

	if !strings.Contains(got, "test-version-123") {
		t.Errorf("expected output to contain 'test-version-123', got: %s", got)
	}
}

func TestVersionCommandFormat(t *testing.T) {
	oldVersion := Version
	defer func() { Version = oldVersion }()

	Version = "dev"
	got := runVersionCmd(t)

	if !strings.HasPrefix(got, "rela version ") {
		t.Errorf("expected output to start with 'rela version ', got: %s", got)
	}

	if !strings.HasSuffix(got, "\n") {
		t.Errorf("expected output to end with newline, got: %s", got)
	}
}
