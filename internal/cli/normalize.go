package cli

import (
	"context"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/markdown"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// NormalizeCmd normalizes markdown headers in entity files.
type NormalizeCmd struct {
	Type   string `arg:"" optional:"" help:"Entity type to normalize (optional)."`
	DryRun bool   `name:"dry-run" help:"Preview changes without writing."`
}

// Run dispatches `rela normalize [type]`.
func (c *NormalizeCmd) Run(ctx context.Context, svc *cliServices) error {
	st := svc.Store()

	q := store.EntityQuery{}
	if c.Type != "" {
		resolvedType, _, err := resolveEntityType(svc.Meta(), c.Type)
		if err != nil {
			return err
		}
		q.Type = resolvedType
	}

	var entities []*entity.Entity
	for e, err := range st.ListEntities(ctx, q) {
		if err != nil {
			return err
		}
		entities = append(entities, e)
	}

	if len(entities) == 0 {
		out.WriteMessage("No entities found")
		return nil
	}

	modified := 0
	for _, e := range entities {
		normalized := markdown.NormalizeHeaders(e.Content)
		if normalized == e.Content {
			continue
		}
		if c.DryRun {
			out.WriteMessage("Would normalize: %s", e.ID)
			modified++
			continue
		}
		e.Content = normalized
		if err := st.UpdateEntity(ctx, e); err != nil {
			out.WriteWarning("Failed to write %s: %v", e.ID, err)
			continue
		}
		modified++
		if verbose {
			out.WriteMessage("Normalized: %s", e.ID)
		}
	}

	if c.DryRun {
		out.WriteMessage("Dry run: %d entities would be modified", modified)
	} else {
		out.WriteSuccess("Normalized %d entities", modified)
	}
	return nil
}
