package cli

import (
	"testing"
)

// update_test.go covers only CLI-level concerns. The property flag
// parsing itself is verified by TestParsePropertyFlag in create_test.go
// (shared helper), and entity property mutation is covered by the
// entity package tests — no need to duplicate either here.

func TestUpdateCmd_PropertyFlagExists(t *testing.T) {
	flag := updateCmd.Flags().Lookup("property")
	if flag == nil {
		t.Fatal("update command should have --property flag")
	}
	if flag.Shorthand != "P" {
		t.Errorf("--property flag shorthand = %q, want %q", flag.Shorthand, "P")
	}
}
