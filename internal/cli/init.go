package cli

import (
	"os"

	"github.com/Sourcehaven-BV/rela/internal/projectsetup"
)

// InitCmd initializes a new rela project.
type InitCmd struct{}

// Run executes `rela init`.
func (c *InitCmd) Run() error {
	targetDir := projectPath
	if targetDir == "" {
		targetDir = os.Getenv("RELA_PROJECT")
	}

	result, err := projectsetup.Initialize(targetDir)
	if err != nil {
		return err
	}

	out.WriteSuccess("Initialized rela project in %s", result.Root)
	out.WriteMessage("  Created metamodel.yaml")
	out.WriteMessage("  Created entities/ directory")
	out.WriteMessage("  Created relations/ directory")
	out.WriteMessage("  Created .rela/ directory (gitignored)")
	out.WriteMessage("")
	out.WriteMessage("Next steps:")
	out.WriteMessage("  rela create requirement    # Create a new requirement")
	out.WriteMessage("  rela list requirements     # List all requirements")

	return nil
}
