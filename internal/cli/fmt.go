package cli

import (
	"context"

	"github.com/Sourcehaven-BV/rela/internal/errors"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// FmtCmd formats entity and relation files.
type FmtCmd struct {
	Type   string `arg:"" optional:"" help:"Entity type to format (optional)."`
	DryRun bool   `name:"dry-run" help:"Preview changes without writing."`
	Check  bool   `help:"Check if files need formatting (exits 1 if they do)."`
}

// Run dispatches `rela fmt [type]`.
func (c *FmtCmd) Run(ctx context.Context, svc *cliServices) error {
	st := svc.Store()
	f, ok := st.(store.Formatter)
	if !ok {
		out.WriteMessage("The active storage backend does not support formatting.")
		return nil
	}

	dryRun := c.DryRun || c.Check

	q := store.EntityQuery{}
	if c.Type != "" {
		resolvedType, _, err := resolveEntityType(svc.Meta(), c.Type)
		if err != nil {
			return err
		}
		q.Type = resolvedType
	}

	modifiedEntities, err := c.formatEntities(ctx, st, f, q, dryRun)
	if err != nil {
		return err
	}

	modifiedRelations := 0
	if c.Type == "" {
		modifiedRelations, err = c.formatRelations(ctx, st, f, dryRun)
		if err != nil {
			return err
		}
	}

	return c.reportFmtResult(modifiedEntities, modifiedRelations)
}

func (c *FmtCmd) formatEntities(
	ctx context.Context,
	st store.Store,
	f store.Formatter,
	q store.EntityQuery,
	dryRun bool,
) (int, error) {
	var entityIDs []string
	for e, err := range st.ListEntities(ctx, q) {
		if err != nil {
			return 0, err
		}
		entityIDs = append(entityIDs, e.ID)
	}
	modified := 0
	for _, id := range entityIDs {
		changed, err := f.FormatEntity(ctx, id, dryRun)
		if err != nil {
			out.WriteWarning("Failed to format %s: %v", id, err)
			continue
		}
		if !changed {
			continue
		}
		modified++
		c.reportFmtItem(id)
	}
	return modified, nil
}

func (c *FmtCmd) formatRelations(
	ctx context.Context,
	st store.Store,
	f store.Formatter,
	dryRun bool,
) (int, error) {
	type relKey struct{ from, typ, to string }
	var relKeys []relKey
	for r, err := range st.ListRelations(ctx, store.RelationQuery{}) {
		if err != nil {
			return 0, err
		}
		relKeys = append(relKeys, relKey{r.From, r.Type, r.To})
	}
	modified := 0
	for _, k := range relKeys {
		changed, err := f.FormatRelation(ctx, k.from, k.typ, k.to, dryRun)
		if err != nil {
			out.WriteWarning("Failed to format relation %s--%s--%s: %v", k.from, k.typ, k.to, err)
			continue
		}
		if !changed {
			continue
		}
		modified++
		c.reportFmtItem(k.from + "--" + k.typ + "--" + k.to)
	}
	return modified, nil
}

func (c *FmtCmd) reportFmtItem(id string) {
	switch {
	case c.Check:
		out.WriteMessage("Needs formatting: %s", id)
	case c.DryRun:
		out.WriteMessage("Would format: %s", id)
	case verbose:
		out.WriteMessage("Formatted: %s", id)
	}
}

func (c *FmtCmd) reportFmtResult(modifiedEntities, modifiedRelations int) error {
	totalModified := modifiedEntities + modifiedRelations
	if c.Check {
		if totalModified > 0 {
			out.WriteMessage("%d files need formatting (%d entities, %d relations)",
				totalModified, modifiedEntities, modifiedRelations)
			return errors.NewExitError(1)
		}
		out.WriteSuccess("All files are properly formatted")
		return nil
	}
	if c.DryRun {
		out.WriteMessage("Dry run: %d files would be formatted (%d entities, %d relations)",
			totalModified, modifiedEntities, modifiedRelations)
	} else {
		out.WriteSuccess("Formatted %d files (%d entities, %d relations)",
			totalModified, modifiedEntities, modifiedRelations)
	}
	return nil
}
