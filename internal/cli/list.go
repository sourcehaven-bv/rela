package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/filter"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/predicate"
	"github.com/Sourcehaven-BV/rela/internal/predicatefns"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// ListCmd lists entities, optionally filtered by type, --where/--filter, and sorted.
type ListCmd struct {
	Type   string   `arg:"" optional:"" help:"Entity type (singular or plural; alias allowed)."`
	Where  []string `help:"Filter by property, legacy syntax (repeatable; e.g. --where status=draft). Deprecated: prefer --filter."`
	Filter string   `help:"Filter by a predicate expression (e.g. --filter \"entity.status == 'ready' and entity.priority ~= 'low'\")."`
	Sort   string   `help:"Sort by property (or 'id')."`
	Desc   bool     `help:"Sort descending."`
}

// Run dispatches `rela list [type]`.
func (c *ListCmd) Run(ctx context.Context, svc *readServices) error {
	meta := svc.Meta
	entityTypeName, q, err := resolveListType(meta, c.Type)
	if err != nil {
		return err
	}

	entities, err := collectListEntities(ctx, svc.Store, q)
	if err != nil {
		return err
	}

	entities, err = applyListFilters(ctx, entities, c.Where, c.Filter, entityTypeName, meta)
	if err != nil {
		return err
	}

	if err := applyListSort(entities, c.Sort, c.Desc, entityTypeName, meta); err != nil {
		return err
	}

	if len(entities) == 0 {
		out.WriteMessage("No entities found")
		return nil
	}
	return out.WriteEntitiesWithSummary(entities)
}

func resolveListType(meta *metamodel.Metamodel, typeName string) (string, store.EntityQuery, error) {
	q := store.EntityQuery{}
	if typeName == "" {
		return "", q, nil
	}
	resolvedType, err := resolveEntityType(meta, typeName)
	if err != nil {
		return "", q, err
	}
	q.Type = resolvedType
	return resolvedType, q, nil
}

func collectListEntities(ctx context.Context, st store.Store, q store.EntityQuery) ([]*entity.Entity, error) {
	var entities []*entity.Entity
	for e, err := range st.ListEntities(ctx, q) {
		if err != nil {
			return nil, err
		}
		entities = append(entities, e)
	}
	return entities, nil
}

// applyListFilters filters entities by --filter (a predicate expression)
// and/or --where (legacy filter strings, transpiled to predicate). Both
// compile once through a metamodel-scoped predicatefns.Evaluator and
// evaluate per entity. --where and --filter may be combined (ANDed).
func applyListFilters(
	ctx context.Context,
	entities []*entity.Entity,
	where []string,
	filterExpr string,
	entityTypeName string,
	meta *metamodel.Metamodel,
) ([]*entity.Entity, error) {
	if len(where) == 0 && filterExpr == "" {
		return entities, nil
	}
	if entityTypeName == "" {
		return nil, errors.New("--where/--filter require specifying an entity type")
	}

	ev := predicatefns.NewEvaluator(meta, time.Now())
	var progs []*predicate.Program

	if len(where) > 0 {
		fmt.Fprintln(os.Stderr, "warning: --where is deprecated; prefer --filter with a predicate expression")
		filters, err := filter.ParseAll(where)
		if err != nil {
			return nil, fmt.Errorf("invalid --where filter: %w", err)
		}
		prog, err := ev.CompileFilter(entityTypeName, filters)
		if err != nil {
			return nil, fmt.Errorf("invalid --where filter: %w", err)
		}
		progs = append(progs, prog)
	}
	if filterExpr != "" {
		prog, err := ev.Compile(entityTypeName, filterExpr)
		if err != nil {
			return nil, fmt.Errorf("invalid --filter expression: %w", err)
		}
		progs = append(progs, prog)
	}

	var filtered []*entity.Entity
	for _, e := range entities {
		match := true
		for _, prog := range progs {
			ok, err := ev.Matches(ctx, prog, e.Type, e.ID, e.Properties)
			if err != nil {
				return nil, fmt.Errorf("filter error: %w", err)
			}
			if !ok {
				match = false
				break
			}
		}
		if match {
			filtered = append(filtered, e)
		}
	}
	return filtered, nil
}

func applyListSort(
	entities []*entity.Entity,
	sortKey string,
	desc bool,
	entityTypeName string,
	meta *metamodel.Metamodel,
) error {
	if sortKey == "" {
		filter.SortByID(entities, storeEntityRecord, desc)
		return nil
	}
	if entityTypeName == "" {
		return errors.New("--sort requires specifying an entity type")
	}
	entityDef, ok := meta.GetEntityDef(entityTypeName)
	if !ok {
		return fmt.Errorf("unknown entity type: %s", entityTypeName)
	}
	if sortKey == "id" {
		filter.SortByID(entities, storeEntityRecord, desc)
		return nil
	}
	propDef, ok := entityDef.Properties[sortKey]
	if !ok {
		return fmt.Errorf("unknown property %q for entity type %q", sortKey, entityTypeName)
	}
	filter.Sort(entities, storeEntityRecord, sortKey, &propDef, meta, desc)
	return nil
}
