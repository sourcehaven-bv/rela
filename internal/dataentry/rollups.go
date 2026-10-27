package dataentry

import (
	"context"
	"log/slog"
	"maps"
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
}

// newRollupSource returns the roll-up source over app's store and field
// resolver.
func newRollupSource(app *App) rollupSource {
	return rollupSource{
		schema: app.State,
		st:     app.store,
		fields: func() visibility.FieldScoper { return app.fieldResolver },
	}
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
// It costs a constant number of store queries per roll-up column, whatever
// len(rows) is. The result is per principal: never cache it across requests.
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
			// The rows are still served, without values: the names are
			// stripped below either way.
			slog.WarnContext(ctx, "roll-ups not computed", "type", entityType, "error", err)
		} else {
			refs := make([]entityPkg.Ref, len(rows))
			for i, row := range rows {
				refs[i] = row.ref
			}
			values = r.Values(ctx, entityType, refs, world, want)
		}
	}
	for _, row := range rows {
		visibility.MergeRollups(row.props, names, values[row.ref.ID])
	}
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

// rollupsParam asks a collection response to carry every roll-up property
// of its rows (TKT-6I2OPG). A roll-up costs store queries per column per
// page, so a list row carries none unless asked: by this parameter, or by
// naming (list_id) a configured list that shows a roll-up as a column, which
// then carries those.
const rollupsParam = "rollups"

// listRollups returns which roll-ups a collection read computes, or nil for
// none.
func listRollups(query map[string][]string, cfg *Config) func(string) bool {
	if all, _ := strconv.ParseBool(queryGet(query, rollupsParam)); all {
		return func(string) bool { return true }
	}
	id := queryGet(query, listIDParam)
	if id == "" || cfg == nil {
		return nil
	}
	list, ok := cfg.Lists[id]
	if !ok {
		return nil
	}
	shown := map[string]bool{}
	for _, c := range list.Columns {
		if c.Property != "" {
			shown[c.Property] = true
		}
	}
	if len(shown) == 0 {
		return nil
	}
	return func(p string) bool { return shown[p] }
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
