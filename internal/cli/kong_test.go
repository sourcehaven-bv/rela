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

// TestRenderRequiresProject pins that `render` is registered in requiresProject.
// A command whose Run binds *readServices but is missing here never gets the
// binding wired, so kong fails at invocation with "couldn't find binding of type
// *cli.readServices" — a runtime-only failure that Run-level unit tests (which
// pass svc directly) can't catch.
func TestRenderRequiresProject(t *testing.T) {
	if !requiresProject("render <id>") {
		t.Error("render must require a project (it binds *readServices); add it to requiresProject")
	}
}
