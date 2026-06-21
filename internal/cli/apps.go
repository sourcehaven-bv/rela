package cli

import (
	"os"

	"github.com/Sourcehaven-BV/rela/internal/projectsetup"
)

// AppsCmd groups custom-app management subcommands.
type AppsCmd struct {
	New AppsNewCmd `cmd:"" help:"Scaffold a new custom app folder (apps/<id>/index.html)."`
}

// AppsNewCmd scaffolds a starter app under apps/<id>/.
type AppsNewCmd struct {
	ID string `arg:"" help:"App id — lowercase letters, digits, '-', '_' (becomes the folder name and /app/<id> route)."`
}

// Run executes `rela apps new <id>`. Self-discovers the project root (no graph
// services needed — it only writes files).
func (c *AppsNewCmd) Run() error {
	startDir := projectPath
	if startDir == "" {
		startDir = os.Getenv("RELA_PROJECT")
	}

	result, err := projectsetup.ScaffoldApp(startDir, c.ID)
	if err != nil {
		return err
	}

	out.WriteSuccess("Created app %q", result.ID)
	out.WriteMessage("  %s", result.IndexAbs)
	out.WriteMessage("")
	out.WriteMessage("Next steps:")
	out.WriteMessage("  - Edit %s", result.IndexAbs)
	out.WriteMessage("  - Run the data-entry server and open /app/%s", result.ID)
	out.WriteMessage("  - The app calls rela.list/get/create/... via window.rela; see the")
	out.WriteMessage("    Custom apps section of the data-entry guide for the full bridge API.")
	return nil
}
