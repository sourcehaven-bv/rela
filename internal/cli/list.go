package cli

import (
	"context"
	"errors"
	"fmt"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/filter"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// ListCmd lists entities, optionally filtered by type, --where, and sorted.
type ListCmd struct {
	Type  string   `arg:"" optional:"" help:"Entity type (singular or plural; alias allowed)."`
	Where []string `help:"Filter by property (repeatable; e.g. --where status=draft)."`
	Sort  string   `help:"Sort by property (or 'id')."`
	Desc  bool     `help:"Sort descending."`
}

// Run dispatches `rela list [type]`.
func (c *ListCmd) Run(ctx context.Context, svc *cliServices) error {
	meta := svc.Meta()
	entityTypeName, q, err := resolveListType(meta, c.Type)
	if err != nil {
		return err
	}

	entities, err := collectListEntities(ctx, svc.Store(), q)
	if err != nil {
		return err
	}

	entities, err = applyListFilters(entities, c.Where, entityTypeName, meta)
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
	resolvedType, _, err := resolveEntityType(meta, typeName)
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

func applyListFilters(
	entities []*entity.Entity,
	where []string,
	entityTypeName string,
	meta *metamodel.Metamodel,
) ([]*entity.Entity, error) {
	if len(where) == 0 {
		return entities, nil
	}
	if entityTypeName == "" {
		return nil, errors.New("--where filters require specifying an entity type")
	}
	entityDef, ok := meta.GetEntityDef(entityTypeName)
	if !ok {
		return nil, fmt.Errorf("unknown entity type: %s", entityTypeName)
	}
	filters, err := filter.ParseAll(where)
	if err != nil {
		return nil, fmt.Errorf("invalid filter: %w", err)
	}
	for _, f := range filters {
		if _, ok := entityDef.Properties[f.Property]; !ok {
			return nil, fmt.Errorf("unknown property %q for entity type %q", f.Property, entityTypeName)
		}
	}
	var filtered []*entity.Entity
	for _, e := range entities {
		matches, err := filter.MatchAll(storeEntityRecord(e), filters, entityDef, meta)
		if err != nil {
			return nil, fmt.Errorf("filter error: %w", err)
		}
		if matches {
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
