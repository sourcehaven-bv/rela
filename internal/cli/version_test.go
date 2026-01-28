package cli

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestVersionCommand(t *testing.T) {
	// Save original Version value
	oldVersion := Version
	defer func() {
		Version = oldVersion
	}()

	// Set a test version
	Version = "1.2.3-test"

	// Find the version command
	var versionCmd *cobra.Command
	for _, cmd := range rootCmd.Commands() {
		if cmd.Name() == "version" {
			versionCmd = cmd
			break
		}
	}

	if versionCmd == nil {
		t.Fatal("version command not found")
	}

	// Capture stdout
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	// Run the command
	versionCmd.Run(versionCmd, []string{})

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	// Check output
	expected := "rela version 1.2.3-test\n"
	if output != expected {
		t.Errorf("version output = %q, want %q", output, expected)
	}
}

func TestVersionCommandContainsVersion(t *testing.T) {
	// Save original Version value
	oldVersion := Version
	defer func() {
		Version = oldVersion
	}()

	Version = "test-version-123"

	// Find the version command
	var versionCmd *cobra.Command
	for _, cmd := range rootCmd.Commands() {
		if cmd.Name() == "version" {
			versionCmd = cmd
			break
		}
	}

	if versionCmd == nil {
		t.Fatal("version command not found")
	}

	// Capture stdout
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	versionCmd.Run(versionCmd, []string{})

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	// Check that output contains the version
	if !strings.Contains(output, "test-version-123") {
		t.Errorf("expected output to contain 'test-version-123', got: %s", output)
	}
}

func TestVersionCommandFormat(t *testing.T) {
	oldVersion := Version
	defer func() {
		Version = oldVersion
	}()

	Version = "dev"

	var versionCmd *cobra.Command
	for _, cmd := range rootCmd.Commands() {
		if cmd.Name() == "version" {
			versionCmd = cmd
			break
		}
	}

	if versionCmd == nil {
		t.Fatal("version command not found")
	}

	// Capture stdout
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	versionCmd.Run(versionCmd, []string{})

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	// Verify format: "rela version <version>\n"
	if !strings.HasPrefix(output, "rela version ") {
		t.Errorf("expected output to start with 'rela version ', got: %s", output)
	}

	if !strings.HasSuffix(output, "\n") {
		t.Errorf("expected output to end with newline, got: %s", output)
	}
}

func TestRootCommandHasVersionFlag(t *testing.T) {
	// The rootCmd should have a version set
	if rootCmd.Version == "" {
		t.Error("rootCmd.Version is empty, expected a version string")
	}

	// The version flag should be available
	versionFlag := rootCmd.Flag("version")
	if versionFlag != nil {
		// Version flag exists (this is automatically added by Cobra when Version is set)
		t.Logf("version flag exists: %v", versionFlag.Name)
	}
}
