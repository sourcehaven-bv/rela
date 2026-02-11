package cli

import (
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// TestNoShorthandConflicts ensures persistent flags don't conflict with local flags.
// This prevents bugs like -p being used for both --project (persistent) and --priority (local).
func TestNoShorthandConflicts(t *testing.T) {
	// Collect all persistent flag shorthands from root
	persistentShorthands := make(map[string]string) // shorthand -> flag name
	rootCmd.PersistentFlags().VisitAll(func(f *pflag.Flag) {
		if f.Shorthand != "" {
			persistentShorthands[f.Shorthand] = f.Name
		}
	})

	// Check each subcommand for conflicts
	for _, cmd := range rootCmd.Commands() {
		checkCommandForConflicts(t, cmd, persistentShorthands)
	}
}

func checkCommandForConflicts(t *testing.T, cmd *cobra.Command, parentShorthands map[string]string) {
	t.Helper()

	// Check local flags against parent persistent flags
	// Same name shadowing is OK (e.g., local --output shadows persistent --output)
	// Different name with same shorthand is a conflict
	cmd.Flags().VisitAll(func(f *pflag.Flag) {
		if f.Shorthand != "" {
			if parentFlag, exists := parentShorthands[f.Shorthand]; exists && parentFlag != f.Name {
				t.Errorf("command %q: flag --%s uses shorthand -%s which conflicts with persistent flag --%s",
					cmd.Name(), f.Name, f.Shorthand, parentFlag)
			}
		}
	})

	// Build combined shorthands for checking nested subcommands
	combinedShorthands := make(map[string]string)
	for k, v := range parentShorthands {
		combinedShorthands[k] = v
	}
	cmd.PersistentFlags().VisitAll(func(f *pflag.Flag) {
		if f.Shorthand != "" {
			combinedShorthands[f.Shorthand] = f.Name
		}
	})

	// Recurse into subcommands
	for _, subCmd := range cmd.Commands() {
		checkCommandForConflicts(t, subCmd, combinedShorthands)
	}
}

func TestRootCmdProjectFlag(t *testing.T) {
	// Verify the project flag is registered
	flag := rootCmd.PersistentFlags().Lookup("project")
	if flag == nil {
		t.Fatal("expected --project flag to be registered")
	}

	// Verify no shorthand (removed to avoid conflict with --priority on create/update)
	if flag.Shorthand != "" {
		t.Errorf("expected no shorthand, got %q", flag.Shorthand)
	}

	// Verify default value is empty (auto-detect)
	if flag.DefValue != "" {
		t.Errorf("expected empty default value, got %q", flag.DefValue)
	}
}
