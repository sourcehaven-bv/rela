package cli

import (
	"testing"

	"github.com/alecthomas/kong"
)

// TestCLIStructBuilds asserts the whole CLI grammar (all subcommands + their
// flags) is well-formed: kong.New panics/errors on a duplicate short flag or
// other malformed tag. A plain unit test that calls a command's Run method
// bypasses kong parsing entirely, so this is the guard that catches a flag
// collision (e.g. a subcommand reusing the global -o) at test time rather than
// on the first real invocation. Regression for the RenderCmd -o collision.
func TestCLIStructBuilds(t *testing.T) {
	var cli CLI
	if _, err := kong.New(&cli, kong.Name("rela"), kong.Vars{"version": Version}); err != nil {
		t.Fatalf("CLI grammar is invalid: %v", err)
	}
}
