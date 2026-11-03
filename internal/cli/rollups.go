package cli

import (
	"context"
	"log/slog"
	"maps"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/rollup"
	"github.com/Sourcehaven-BV/rela/internal/visibility"
)

// withRollups returns entities with their roll-up properties (TKT-6I2OPG)
// computed, one batch per entity type. An entity with roll-ups is replaced
// by a copy; a value stored under a roll-up name is never printed.
//
// The CLI reads at the operator trust boundary, so the related rows are
// read without a gate, as every other CLI read is.
func withRollups(ctx context.Context, svc *readServices, entities []*entity.Entity) []*entity.Entity {
	byType := map[string][]int{}
	for i, e := range entities {
		if len(svc.Meta.RollupNames(e.Type)) > 0 {
			byType[e.Type] = append(byType[e.Type], i)
		}
	}
	if len(byType) == 0 {
		return entities
	}
	var r *visibility.Rollups
	if set, err := rollup.Compile(svc.Meta); err != nil {
		slog.Warn("roll-up properties not computed", "err", err)
	} else if r, err = visibility.NewRollups(
		set, svc.Store, visibility.AllowAllReadQueries{}, visibility.NopRedactor{},
	); err != nil {
		slog.Warn("roll-up properties not computed", "err", err)
	}
	out := make([]*entity.Entity, len(entities))
	copy(out, entities)
	for typ, idx := range byType {
		var values map[string]map[string]any
		if r != nil {
			refs := make([]entity.Ref, len(idx))
			for j, i := range idx {
				refs[j] = entities[i].Ref()
			}
			values = r.Values(ctx, typ, refs, svc.World, nil)
		}
		names := svc.Meta.RollupNames(typ)
		for _, i := range idx {
			cp := *entities[i]
			cp.Properties = visibility.MergeRollups(maps.Clone(cp.Properties), names, values[cp.ID])
			out[i] = &cp
		}
	}
	return out
}
