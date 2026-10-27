package mcp

import (
	"context"
	"maps"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/visibility"
)

// RollupReader computes roll-up properties (TKT-6I2OPG) under the caller's
// access. The wiring passes [visibility.Rollups] from the same gated read
// bundle as [Deps.Store].
type RollupReader interface {
	Values(
		ctx context.Context, entityType string, rows []entity.Ref, world store.WorldScope, want func(string) bool,
	) map[string]map[string]any
}

// withRollups returns e's properties for the wire: a copy with the stored
// keys named like a roll-up removed and the computed roll-ups added. Related
// rows are read in world.
func withRollups(ctx context.Context, d Deps, e *entity.Entity, world store.WorldScope) map[string]any {
	names := d.Meta.RollupNames(e.Type)
	if len(names) == 0 {
		return e.Properties
	}
	var values map[string]any
	if d.Related.Rollups != nil {
		values = d.Related.Rollups.Values(ctx, e.Type, []entity.Ref{e.Ref()}, world, nil)[e.ID]
	}
	return visibility.MergeRollups(maps.Clone(e.Properties), names, values)
}
