package cli

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
)

// AttachCmd attaches one or more files to an entity property.
type AttachCmd struct {
	Property string   `short:"P" help:"Property to attach file(s) to (defaults to first file-type property)."`
	EntityID string   `arg:"" name:"entity-id" help:"Target entity ID."`
	Files    []string `arg:"" help:"File path(s) (globs supported)."`
}

// Run dispatches `rela attach <entity-id> <file>...`.
func (c *AttachCmd) Run(ctx context.Context, svc *cliServices) error {
	var attached int
	for _, filePath := range c.Files {
		matches, err := filepath.Glob(filePath)
		if err != nil {
			return fmt.Errorf("invalid glob pattern %q: %w", filePath, err)
		}
		if len(matches) == 0 {
			matches = []string{filePath}
		}
		for _, match := range matches {
			absPath, err := filepath.Abs(match)
			if err != nil {
				return fmt.Errorf("invalid path %q: %w", match, err)
			}
			result, err := svc.AttachFile(ctx, c.EntityID, absPath, c.Property)
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
}
