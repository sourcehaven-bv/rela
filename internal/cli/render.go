package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/natsort"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/transform"
)

// RenderCmd renders one entity to a file via a registered transform (e.g. pdf,
// docx). The transforms come from the metamodel `transforms:` registry; this
// command wires the built-in entity renderer to the transform engine. Unlike
// `export` (a JSON/CSV/YAML data dump), `render` produces a presentation
// document converted by an external tool.
type RenderCmd struct {
	ID        string `arg:"" help:"Entity ID to render."`
	Transform string `short:"t" required:"" help:"Registered transform name (e.g. pdf, docx)."`
	Out       string `short:"o" required:"" help:"Output file path."`
}

// Run dispatches `rela render <id> --transform <name> --out <file>`.
func (c *RenderCmd) Run(ctx context.Context, svc *readServices) error {
	reg := transform.RegistryFromMetamodel(svc.Meta)
	if _, ok := reg[c.Transform]; !ok {
		return fmt.Errorf("unknown transform %q (configured transforms: %s)",
			c.Transform, transformNames(reg))
	}

	e, err := svc.Store.GetEntity(ctx, c.ID)
	if err != nil {
		return fmt.Errorf("entity %q: %w", c.ID, err)
	}

	renderer := transform.EntityRenderer{
		Entity:    e,
		Meta:      svc.Meta,
		Relations: c.relationGroups(ctx, svc, e.ID),
	}

	eng, err := transform.NewEngine(reg)
	if err != nil {
		return err
	}
	res, err := eng.Run(ctx, c.Transform, renderer)
	if err != nil {
		var unknown transform.UnknownTransformError
		if errors.As(err, &unknown) {
			return err // already a clear message
		}
		return fmt.Errorf("render %q as %q: %w", c.ID, c.Transform, err)
	}

	if werr := os.WriteFile(c.Out, res.Data, 0o644); werr != nil {
		return fmt.Errorf("write %q: %w", c.Out, werr)
	}
	if !quiet {
		fmt.Printf("Wrote %s (%s, %d bytes)\n", c.Out, res.Produces, len(res.Data))
	}
	return nil
}

// relationGroups resolves the entity's outgoing relations into display groups
// (relation label + neighbor display titles) for the entity renderer. The CLI
// runs as the operator (no ACL scoping), so it shows all neighbors.
func (c *RenderCmd) relationGroups(ctx context.Context, svc *readServices, id string) []transform.RelationGroup {
	byType := map[string][]string{}
	q := store.RelationQuery{EntityID: id, Direction: store.DirectionOutgoing}
	for rel, err := range svc.Store.ListRelations(ctx, q) {
		if err != nil {
			break
		}
		title := rel.To
		if node, gerr := svc.Store.GetEntity(ctx, rel.To); gerr == nil {
			title = displayTitleOrTitle(svc.Meta, node)
		}
		byType[rel.Type] = append(byType[rel.Type], title)
	}

	types := make([]string, 0, len(byType))
	for t := range byType {
		types = append(types, t)
	}
	sort.Strings(types)

	groups := make([]transform.RelationGroup, 0, len(types))
	for _, t := range types {
		label := t
		if def, ok := svc.Meta.GetRelationDef(t); ok && def.Label != "" {
			label = def.Label
		}
		neighbors := byType[t]
		natsort.Strings(neighbors)
		groups = append(groups, transform.RelationGroup{Label: label, Neighbors: neighbors})
	}
	return groups
}

// displayTitleOrTitle resolves an entity's human title, falling back from the
// metamodel DisplayTitle (whose fallback is the ID) to the raw title property
// before the ID — matching the entity renderer's title precedence.
func displayTitleOrTitle(meta *metamodel.Metamodel, e *entity.Entity) string {
	if t := meta.DisplayTitle(e.ID, e.Type, e.Properties); t != "" && t != e.ID {
		return t
	}
	if t := e.Title(); t != "" {
		return t
	}
	return e.ID
}

func transformNames(reg transform.Registry) string {
	names := make([]string, 0, len(reg))
	for _, nd := range reg.FromMarkdown() {
		names = append(names, nd.Name)
	}
	if len(names) == 0 {
		return "(none configured)"
	}
	return strings.Join(names, ", ")
}
