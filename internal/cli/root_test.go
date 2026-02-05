package cli

import (
	"testing"
)

func TestRootCmdProjectFlag(t *testing.T) {
	// Verify the project flag is registered
	flag := rootCmd.PersistentFlags().Lookup("project")
	if flag == nil {
		t.Fatal("expected --project flag to be registered")
	}

	// Verify short flag
	if flag.Shorthand != "p" {
		t.Errorf("expected shorthand 'p', got %q", flag.Shorthand)
	}

	// Verify default value is empty (auto-detect)
	if flag.DefValue != "" {
		t.Errorf("expected empty default value, got %q", flag.DefValue)
	}
}
