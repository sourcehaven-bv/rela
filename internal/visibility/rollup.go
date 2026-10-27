package visibility

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/rollup"
	"github.com/Sourcehaven-BV/rela/internal/search"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// Rollups computes roll-up properties (TKT-6I2OPG) under the reader's
// access, gate before fold:
//
//   - Rows: the related rows are the reader's own read query for the related
//     type ([ReadQueryProvider]), in the request's world, so a related entity
//     the reader cannot read never counts.
//   - Fields: a related row counts only when every property the roll-up reads
//     on it (where keys, the min/max/sum property, latest_by) is visible to
//     the reader there. A latest_by roll-up whose winning row hides the pick
//     has no value; it never falls back to an older row. The verdicts come
//     from the reader's compiled field scope ([FieldScoper]), the one search
//     uses, and are evaluated by the store inside the aggregate.
//
// The result is what the reader could compute from the rows and fields they
// can see, so it discloses no hidden value. It is per request: never cache
// it across principals.
//
// One store call per roll-up column per batch of sources, whatever the
// batch size.
type Rollups struct {
	set    *rollup.Set
	st     store.Store
	rows   ReadQueryProvider
	fields FieldScoper
}

// NewRollups builds the roll-up reader.
//
// Nil: st, rows and fields rejected — a missing gate would aggregate
// ungated. set accepted; a nil set has no roll-ups.
func NewRollups(set *rollup.Set, st store.Store, rows ReadQueryProvider, fields FieldScoper) (*Rollups, error) {
	switch {
	case st == nil:
		return nil, errors.New("visibility: NewRollups: store is required")
	case rows == nil:
		return nil, errors.New("visibility: NewRollups: read query provider is required")
	case fields == nil:
		return nil, errors.New("visibility: NewRollups: field scoper is required")
	}
	return &Rollups{set: set, st: st, rows: rows, fields: fields}, nil
}

// Names returns the roll-up properties of entityType, sorted.
//
// Nil: accepted; a nil Rollups has none.
func (r *Rollups) Names(entityType string) []string {
	if r == nil {
		return nil
	}
	cols := r.set.For(entityType)
	out := make([]string, len(cols))
	for i, c := range cols {
		out[i] = c.Property
	}
	return out
}

// Values computes the roll-ups of entityType for the served rows, keyed by
// id then property, with related rows read in world. rows are rows the
// reader may read, all of entityType. want, when non-nil, limits the
// properties computed. A roll-up without a value (no related row supplied
// one) is absent; count and exists always have one. A roll-up that a
// `visible:` grant on entityType hides on a row is absent on that row.
//
// A roll-up the reader's policy cannot express as a store query, or whose
// store call fails, is left out and logged: an absent value, never an
// ungated one. The caller's ctx must carry the bound request, as for every
// gated read.
//
// Nil: accepted; a nil Rollups computes nothing.
func (r *Rollups) Values(
	ctx context.Context, entityType string, rows []entity.Ref, world store.WorldScope, want func(string) bool,
) map[string]map[string]any {
	out := map[string]map[string]any{}
	if r == nil || len(rows) == 0 {
		return out
	}
	cols := r.set.For(entityType)
	if len(cols) == 0 {
		return out
	}
	ids := make([]string, 0, len(rows))
	for _, ref := range rows {
		if !slices.Contains(ids, ref.ID) {
			ids = append(ids, ref.ID)
		}
	}
	own, err := r.fields.SearchFieldScope(ctx, entityType)
	if err != nil {
		slog.Warn("visibility: roll-ups left out", "type", entityType, "err", err)
		return out
	}
	for _, col := range cols {
		if want != nil && !want(col.Property) {
			continue
		}
		shown, err := r.shownOn(ctx, own, entityType, col.Property, ids)
		if err != nil {
			slog.Warn("visibility: roll-up left out", "type", entityType, "property", col.Property, "err", err)
			continue
		}
		results, err := r.column(ctx, col, ids, world)
		if err != nil {
			slog.Warn("visibility: roll-up left out", "type", entityType, "property", col.Property, "err", err)
			continue
		}
		for _, ref := range rows {
			id := ref.ID
			v, ok := col.ValueOf(results[id])
			if !ok || !shown(store.EntityHeader{ID: ref.ID, Face: ref.Face}) {
				continue
			}
			if out[id] == nil {
				out[id] = map[string]any{}
			}
			out[id][col.Property] = v
		}
	}
	return out
}

// shownOn is the test for "own shows prop on this row" over ids: one store
// query per grant that conditionally shows prop, none when prop is not
// gated.
func (r *Rollups) shownOn(
	ctx context.Context, own *search.FieldScope, entityType, prop string, ids []string,
) (func(store.EntityHeader) bool, error) {
	c, gated := visibleWhen(own, prop)
	if !gated {
		return func(store.EntityHeader) bool { return true }, nil
	}
	return store.HoldingRows(ctx, r.st, entityType, []store.RowCondition{c}, ids)
}

// AllowAllReadQueries is the [ReadQueryProvider] for a project without an
// ACL policy: every type is readable in full. Pass it only at a wiring site
// that has no policy; it is never inferred from identity.
type AllowAllReadQueries struct{}

// ReadQueryFor implements [ReadQueryProvider].
func (AllowAllReadQueries) ReadQueryFor(context.Context, string) (acl.ReadQueryResult, error) {
	return acl.ReadQueryResult{AllowAll: true}, nil
}

// ReadQueryFunc adapts a function to [ReadQueryProvider].
type ReadQueryFunc func(ctx context.Context, entityType string) (acl.ReadQueryResult, error)

// ReadQueryFor implements [ReadQueryProvider].
func (f ReadQueryFunc) ReadQueryFor(ctx context.Context, entityType string) (acl.ReadQueryResult, error) {
	return f(ctx, entityType)
}

// errNoReadQuery reports a read policy with no store form.
var errNoReadQuery = errors.New("visibility: the read policy for the related type has no store query")

// column runs one roll-up for ids.
func (r *Rollups) column(
	ctx context.Context, col rollup.Column, ids []string, world store.WorldScope,
) (map[string]store.RollupResult, error) {
	rqr, err := r.rows.ReadQueryFor(ctx, col.Target)
	if err != nil {
		return nil, err
	}
	var related store.GraphQuery
	switch {
	case rqr.DenyAll:
		return map[string]store.RollupResult{}, nil
	case rqr.AllowAll:
	case rqr.Query != nil:
		// A copy: the ACL may cache the template per principal, so the
		// slices it shares must not be appended to in place.
		related = *rqr.Query
		related.Props = slices.Clone(related.Props)
		related.Related = slices.Clone(related.Related)
	default:
		return nil, errNoReadQuery
	}
	related.EntityType = col.Target
	related.Faces = store.InWorld(world)
	related.FaceIn = rqr.Faces
	related.Props = append(related.Props, col.Where...)

	fs, err := r.fields.SearchFieldScope(ctx, col.Target)
	if err != nil {
		return nil, fmt.Errorf("field scope: %w", err)
	}
	q := store.RollupQuery{
		Related: related, Relation: col.Relation, SourceIsTo: col.SourceIsTo, Sources: ids,
		Func: col.StoreFunc(), Value: col.Value, By: col.By, Numeric: col.Numeric,
	}
	for _, p := range col.Reads {
		if c, gated := visibleWhen(fs, p); gated {
			q.Require = append(q.Require, c)
		}
	}
	if q.Func == store.RollupLatest {
		if c, gated := visibleWhen(fs, col.Value); gated {
			q.PickRequire = append(q.PickRequire, c)
		}
	}
	return store.Rollup(ctx, r.st, q)
}

// visibleWhen is the row condition under which fs shows prop, and whether
// fs gates prop at all. A gated property no grant lists yields an empty
// condition, which holds for no row.
func visibleWhen(fs *search.FieldScope, prop string) (store.RowCondition, bool) {
	if !fs.Gates(prop) {
		return store.RowCondition{}, false
	}
	var c store.RowCondition
	for _, g := range fs.Grants {
		if slices.Contains(g.Fields, prop) {
			c.Any = append(c.Any, g.When)
		}
	}
	return c, true
}

// MergeRollups writes a row's roll-up values into props, the wire form of
// the row. Every roll-up name is first removed: a value stored under a
// roll-up name (a hand edit, an import, a property later turned into a
// roll-up) is never served. [Rollups.Values] already left out a roll-up
// hidden on the row.
//
// props must be a copy the caller owns, never a store entity's map. A nil
// props is returned as a new map when there is a value to write.
func MergeRollups(props map[string]any, names []string, values map[string]any) map[string]any {
	for _, name := range names {
		delete(props, name)
		if v, ok := values[name]; ok {
			if props == nil {
				props = map[string]any{}
			}
			props[name] = v
		}
	}
	return props
}
