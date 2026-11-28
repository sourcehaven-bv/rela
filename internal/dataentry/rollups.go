package dataentry

import (
	"context"
	"fmt"
	"log/slog"
	"maps"
	"slices"
	"strconv"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	v1 "github.com/Sourcehaven-BV/rela/internal/apiwire/v1"
	entityPkg "github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/visibility"
)

// rollupSource computes roll-up properties (TKT-6I2OPG) for one request.
// The functions here are free functions over it rather than App methods:
// App is at its plimsoll method load line.
type rollupSource struct {
	// schema is the published schema; it is loaded once per call.
	schema func() *Schema
	st     store.Store
	// fields is the `visible:` scope source, late-bound because tests
	// reassign the App's field resolver after construction.
	fields func() visibility.FieldScoper
	// records is the record service of a roll-up over a record type,
	// late-bound because [SetRecords] runs after the source is made. It
	// returns nil when none is installed.
	records func() visibility.RecordAggregator
}

// newRollupSource returns the roll-up source over app's store and field
// resolver.
func newRollupSource(app *App) rollupSource {
	return rollupSource{
		schema: app.State,
		st:     app.store,
		fields: func() visibility.FieldScoper { return app.fieldResolver },
		records: func() visibility.RecordAggregator {
			if app.records == nil || app.records.rollups == nil {
				return nil
			}
			return app.records.rollups
		},
	}
}

// withRecords gives r the record service, when one is installed, so a
// roll-up over a record type has a value.
func (s rollupSource) withRecords(r *visibility.Rollups) *visibility.Rollups {
	if s.records == nil {
		return r
	}
	if agg := s.records(); agg != nil {
		return r.WithRecords(agg, nil)
	}
	return r
}

// rollupRow is one served row to add roll-ups to.
type rollupRow struct {
	ref   entityPkg.Ref
	props map[string]any
}

// add writes the roll-ups of rows, all of entityType, into their wire
// properties, with related rows read in world: related rows under the
// request's read gate, fields under its `visible:` scope. want, when
// non-nil, limits the roll-ups computed; the others are absent. Each props
// map must be the row's wire copy, never a store entity's map.
//
// It costs a constant number of store queries per roll-up column for each
// [rollupChunk] rows. The result is per principal: never cache it across requests.
// A zero rollupSource (a handler a test builds without one) adds nothing.
func (s rollupSource) add(
	ctx context.Context, entityType string, rows []rollupRow, world store.WorldScope, want func(string) bool,
) {
	if len(rows) == 0 || s.schema == nil {
		return
	}
	sch := s.schema()
	if sch == nil {
		return
	}
	names := sch.Meta.RollupNames(entityType)
	if len(names) == 0 {
		return
	}
	var values map[string]map[string]any
	if len(sch.rollups.For(entityType)) > 0 {
		gate := visibility.ReadQueryFunc(func(ctx context.Context, t string) (acl.ReadQueryResult, error) {
			return readGateFromContext(ctx).ReadQuery(ctx, t), nil
		})
		r, err := visibility.NewRollups(sch.rollups, s.st, gate, s.fields())
		if err != nil {
			slog.WarnContext(ctx, "roll-ups not computed", "type", entityType, "error", err)
		} else {
			values = rollupValuesChunked(ctx, s.withRecords(r), entityType, rows, world, want)
		}
	}
	for _, row := range rows {
		visibility.MergeRollups(row.props, names, values[row.ref.ID])
	}
}

// rollupChunk is the most source rows one roll-up statement is asked
// about. A list condition computes roll-ups for its whole candidate
// population, so the chunk bounds the size of each statement's id set.
const rollupChunk = 1000

// rollupValuesChunked is [visibility.Rollups.Values] over rows, one call
// per [rollupChunk] rows.
func rollupValuesChunked(
	ctx context.Context, r *visibility.Rollups, entityType string, rows []rollupRow,
	world store.WorldScope, want func(string) bool,
) map[string]map[string]any {
	values := map[string]map[string]any{}
	for chunk := range slices.Chunk(rows, rollupChunk) {
		refs := make([]entityPkg.Ref, len(chunk))
		for i, row := range chunk {
			refs[i] = row.ref
		}
		maps.Copy(values, r.Values(ctx, entityType, refs, world, want))
	}
	return values
}

// addToEntities replaces each entity in groups by a copy carrying its
// roll-ups, one batch per entity type across all groups. The copies are
// fresh, so an entity shared between groups is never written through.
func (s rollupSource) addToEntities(ctx context.Context, world store.WorldScope, groups ...[]*entityPkg.Entity) {
	if s.schema == nil || s.schema() == nil {
		return
	}
	meta := s.schema().Meta
	byType := map[string][]rollupRow{}
	var order []string
	copies := map[entityPkg.Ref]*entityPkg.Entity{}
	for g, entities := range groups {
		for i, e := range entities {
			if len(meta.RollupNames(e.Type)) == 0 {
				continue
			}
			c, ok := copies[e.Ref()]
			if !ok {
				cp := *e
				cp.Properties = maps.Clone(e.Properties)
				if cp.Properties == nil {
					cp.Properties = map[string]any{}
				}
				c = &cp
				copies[e.Ref()] = c
				if _, seen := byType[e.Type]; !seen {
					order = append(order, e.Type)
				}
				byType[e.Type] = append(byType[e.Type], rollupRow{ref: e.Ref(), props: c.Properties})
			}
			groups[g][i] = c
		}
	}
	for _, typ := range order {
		s.add(ctx, typ, byType[typ], world, nil)
	}
}

// rollupsParam set to false leaves the roll-up properties out of a
// collection response. A list row carries every roll-up of its type by
// default (TKT-6I2OPG), as the entity GET does: a roll-up costs one store
// query per column per page, whatever the page size.
const rollupsParam = "rollups"

// listRollups returns which roll-ups a collection read computes, or nil for
// none.
func listRollups(query map[string][]string) func(string) bool {
	if v := queryGet(query, rollupsParam); v != "" {
		if all, err := strconv.ParseBool(v); err == nil && !all {
			return nil
		}
	}
	return func(string) bool { return true }
}

// queriedRollups returns the roll-ups of typeName that query filters or
// sorts on, or nil when it names none.
func queriedRollups(meta *metamodel.Metamodel, typeName string, query gatedListQuery) map[string]bool {
	names := meta.RollupNames(typeName)
	if len(names) == 0 {
		return nil
	}
	var out map[string]bool
	for _, ref := range listQueryProperties(query.params, query.isRelationKey) {
		if slices.Contains(names, ref.property) {
			if out == nil {
				out = map[string]bool{}
			}
			out[ref.property] = true
		}
	}
	return out
}

// withQueriedRollups runs narrow over copies of entities that carry the
// roll-ups query filters or sorts on, and returns the originals narrow kept,
// in its order. A roll-up is computed per reader and never stored, so the
// list filter and sort see it only this way. It costs one store query per
// queried roll-up per [rollupChunk] rows, before paging; the store cannot
// page by a roll-up, so it never takes the pushdown (listNarrowing.pushdownPlan).
//
// The originals are returned, not the copies: every later step (redaction,
// serialization) reads raw store entities, and the served page computes
// its roll-ups again, for its own rows only.
func withQueriedRollups(
	ctx context.Context, s rollupSource, typeName string, query gatedListQuery, entities []*entityPkg.Entity,
	narrow func([]*entityPkg.Entity) ([]*entityPkg.Entity, error),
) ([]*entityPkg.Entity, error) {
	if s.schema == nil || s.schema() == nil {
		return narrow(entities)
	}
	want := queriedRollups(s.schema().Meta, typeName, query)
	if want == nil {
		return narrow(entities)
	}
	for _, col := range s.schema().rollups.For(typeName) {
		if col.BodyTasks != "" && want[col.Property] {
			return nil, fmt.Errorf("%w: %q counts body tasks and cannot be filtered or sorted on",
				errBadFilter, col.Property)
		}
	}
	if len(entities) == 0 {
		return narrow(entities)
	}
	copies := s.copiesWithRollups(ctx, typeName, entities, want)
	original := make(map[*entityPkg.Entity]*entityPkg.Entity, len(entities))
	for i, c := range copies {
		original[c] = entities[i]
	}
	kept, err := narrow(copies)
	if err != nil {
		return nil, err
	}
	out := make([]*entityPkg.Entity, len(kept))
	for i, c := range kept {
		out[i] = original[c]
	}
	return out, nil
}

// copiesWithRollups returns copies of entities, all of typeName, carrying
// the roll-ups in want as this reader sees them, in the same order. A
// roll-up with no value is absent from its copy, so a condition reads it as
// unset. Every stored value under a roll-up name is dropped.
func (s rollupSource) copiesWithRollups(
	ctx context.Context, typeName string, entities []*entityPkg.Entity, want map[string]bool,
) []*entityPkg.Entity {
	copies := make([]*entityPkg.Entity, len(entities))
	rows := make([]rollupRow, len(entities))
	for i, e := range entities {
		cp := *e
		cp.Properties = maps.Clone(e.Properties)
		if cp.Properties == nil {
			cp.Properties = map[string]any{}
		}
		copies[i] = &cp
		rows[i] = rollupRow{ref: e.Ref(), props: cp.Properties}
	}
	s.add(ctx, typeName, rows, worldScopeFrom(ctx), func(p string) bool { return want[p] })
	return copies
}

// serveRowRollups adds the roll-ups want selects to each row in data, which
// is parallel to rows: one batch per entity type on the page.
func serveRowRollups(
	ctx context.Context, s rollupSource, rows []*entityPkg.Entity, data []v1.Entity, want func(string) bool,
) {
	byType := map[string][]rollupRow{}
	var order []string
	for i, e := range rows {
		if data[i].Properties == nil {
			data[i].Properties = map[string]any{}
		}
		if _, seen := byType[e.Type]; !seen {
			order = append(order, e.Type)
		}
		byType[e.Type] = append(byType[e.Type], rollupRow{ref: e.Ref(), props: data[i].Properties})
	}
	for _, typ := range order {
		s.add(ctx, typ, byType[typ], worldScopeFrom(ctx), want)
	}
}

// copyRollups copies the roll-up values e carries (see
// [rollupSource.addToEntities]) into the wire properties props, which the
// serializer stripped of every roll-up name.
func copyRollups(meta *metamodel.Metamodel, e *entityPkg.Entity, props map[string]any) {
	for _, name := range meta.RollupNames(e.Type) {
		if v, ok := e.Properties[name]; ok {
			props[name] = v
		}
	}
}
