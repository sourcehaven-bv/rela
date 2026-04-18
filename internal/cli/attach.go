package cli

import (
	"errors"
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"
)

var attachProperty string

var attachCmd = &cobra.Command{
	Use:   "attach <entity-id> <file>...",
	Short: "Attach file(s) to an entity",
	Long: `Attach one or more files to an entity.

Files are stored in a content-addressable store using SHA-256 hashes.
Duplicate files are automatically deduplicated.

The --property flag specifies which property to attach the file(s) to.
If not specified, uses the first file-type property defined for the entity type.

Examples:
  rela attach BUG-042 screenshot.png
  rela attach BUG-042 screenshot.png --property screenshot
  rela attach DEC-007 *.pdf --property supporting-docs
  rela attach REQ-001 diagram.png spec.pdf`,
	Args: cobra.MinimumNArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		entityID := args[0]
		filePaths := args[1:]

		var attached int
		for _, filePath := range filePaths {
			// Expand globs
			matches, err := filepath.Glob(filePath)
			if err != nil {
				return fmt.Errorf("invalid glob pattern %q: %w", filePath, err)
			}
			if len(matches) == 0 {
				// No glob match, try as literal path
				matches = []string{filePath}
			}

			for _, match := range matches {
				// Convert to absolute path for reading
				absPath, err := filepath.Abs(match)
				if err != nil {
					return fmt.Errorf("invalid path %q: %w", match, err)
				}

				result, err := ws.AttachFile(entityID, absPath, attachProperty)
				if err != nil {
					return fmt.Errorf("failed to attach %q: %w", match, err)
				}
				out.WriteSuccess("Attached %s → %s", filepath.Base(match), result.Path)
				attached++
			}
		}

		if attached == 0 {
			return errors.New("no files matched")
		}

		return nil
	},
}

func init() {
	attachCmd.Flags().StringVarP(&attachProperty, "property", "P", "", "Property to attach file(s) to")
	rootCmd.AddCommand(attachCmd)
}
