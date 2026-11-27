package visibility

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"time"

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
//     the reader cannot read never counts. On a roll-up over a path, every
//     entity on the path is gated the same way, so one the reader cannot
//     read breaks each path through it.
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
	// recs and now serve roll-ups over record types; see [Rollups.WithRecords].
	recs RecordAggregator
	now  func() time.Time
	// rowCap caps the rows of each walk; zero means [store.AlongRowCap].
	// A nested roll-up shares it over its whole evaluation.
	rowCap int
}

// WithRowCap returns a copy of r whose walks hold at most n rows each; a
// nested roll-up (design section 10) holds at most n over the outer walk
// and its inner roll-ups together. Zero restores [store.AlongRowCap].
//
// Nil: accepted; a nil Rollups stays nil.
func (r *Rollups) WithRowCap(n int) *Rollups {
	if r == nil {
		return nil
	}
	c := *r
	c.rowCap = n
	return &c
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
	if r == nil {
		return map[string]map[string]any{}
	}
	out, _ := r.values(ctx, entityType, rows, world, want, r.rowCap)
	return out
}

// values is [Rollups.Values] with each walk capped at rowCap rows, and the
// problems of the columns it left out joined into err.
func (r *Rollups) values(
	ctx context.Context, entityType string, rows []entity.Ref, world store.WorldScope, want func(string) bool,
	rowCap int,
) (out map[string]map[string]any, err error) {
	out = map[string]map[string]any{}
	if len(rows) == 0 {
		return out, nil
	}
	cols := r.set.For(entityType)
	if len(cols) == 0 {
		return out, nil
	}
	ids := make([]string, 0, len(rows))
	seen := make(map[string]struct{}, len(rows))
	for _, ref := range rows {
		if _, dup := seen[ref.ID]; !dup {
			seen[ref.ID] = struct{}{}
			ids = append(ids, ref.ID)
		}
	}
	own, err := r.fields.SearchFieldScope(ctx, entityType)
	if err != nil {
		slog.Warn("visibility: roll-ups left out", "type", entityType, "err", err)
		return out, err
	}
	var failed []error
	// bodies are the served rows' readable bodies, read once for every
	// body_tasks column of the type.
	var bodies map[entity.Ref]string
	for _, col := range cols {
		if want != nil && !want(col.Property) {
			continue
		}
		shown, err := r.shownOn(ctx, own, entityType, col.Property, ids)
		if err != nil {
			slog.Warn("visibility: roll-up left out", "type", entityType, "property", col.Property, "err", err)
			failed = append(failed, err)
			continue
		}
		var results map[string]store.RollupResult
		var withheld map[string]bool
		switch {
		case col.BodyTasks != "":
			if bodies == nil {
				bodies = readBodiesOrNone(ctx, r.st, entityType, rows)
			}
			results = bodyColumn(col, rows, bodies)
		case col.Records != "":
			results, err = r.recordColumn(ctx, col, ids)
		case col.Siblings != nil:
			results, withheld, err = r.siblingsColumn(ctx, col, ids, world, rowCap)
		case col.Along != nil:
			results, withheld, err = r.alongColumn(ctx, col, ids, world, rowCap)
		default:
			results, err = r.column(ctx, col, ids, world)
		}
		if err != nil {
			slog.Warn("visibility: roll-up left out", "type", entityType, "property", col.Property, "err", err)
			failed = append(failed, err)
			continue
		}
		for _, ref := range rows {
			id := ref.ID
			v, ok := col.ValueOf(results[id])
			if !ok || withheld[id] || !shown(store.EntityHeader{ID: ref.ID, Face: ref.Face}) {
				continue
			}
			if out[id] == nil {
				out[id] = map[string]any{}
			}
			out[id][col.Property] = v
		}
	}
	return out, errors.Join(failed...)
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

// Bind implements [ElementReads]: without a policy there is no request to
// check, so ctx is returned as it is.
func (AllowAllReadQueries) Bind(ctx context.Context) (context.Context, error) { return ctx, nil }

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
	related, denied, err := r.gate(ctx, col.Target, world)
	if err != nil {
		return nil, err
	}
	if denied {
		return map[string]store.RollupResult{}, nil
	}
	related.Props = append(related.Props, col.Where...)
	// Every entity on a path is gated as the related rows are: one the
	// reader cannot read breaks each path through it (TKT-LTSNHD).
	path := make([]store.RollupStep, 0, len(col.Path))
	for _, hop := range col.Path {
		through, hopDenied, hopErr := r.gate(ctx, hop.Target, world)
		if hopErr != nil {
			return nil, hopErr
		}
		if hopDenied {
			return map[string]store.RollupResult{}, nil
		}
		path = append(path, store.RollupStep{Relation: hop.Relation, SourceIsTo: hop.SourceIsTo, Through: through})
	}

	fs, err := r.fields.SearchFieldScope(ctx, col.Target)
	if err != nil {
		return nil, fmt.Errorf("field scope: %w", err)
	}
	q := store.RollupQuery{
		Related: related, Path: path, Relation: col.Relation, SourceIsTo: col.SourceIsTo, Sources: ids,
		Func: col.StoreFunc(), Value: col.Value, By: col.By, Numeric: col.Numeric, Decimal: col.Decimal,
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

// gate is the reader's read query for entityType in world: the rows of that
// type the reader may read. denied reports that the reader reads none.
func (r *Rollups) gate(
	ctx context.Context, entityType string, world store.WorldScope,
) (q store.GraphQuery, denied bool, err error) {
	rqr, err := r.rows.ReadQueryFor(ctx, entityType)
	if err != nil {
		return store.GraphQuery{}, false, fmt.Errorf("read query for %s: %w", entityType, err)
	}
	switch {
	case rqr.DenyAll:
		return store.GraphQuery{}, true, nil
	case rqr.AllowAll:
	case rqr.Query != nil:
		// A copy: the ACL may cache the template per principal, so the
		// slices it shares must not be appended to in place.
		q = *rqr.Query
		q.Props = slices.Clone(q.Props)
		q.Related = slices.Clone(q.Related)
	default:
		return store.GraphQuery{}, false, errNoReadQuery
	}
	q.EntityType = entityType
	q.Faces = store.InWorld(world)
	q.FaceIn = rqr.Faces
	return q, false, nil
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

// readBodies reads the bodies of rows, one store call per distinct face, for
// the body_tasks roll-ups (TKT-SQ8O0EQ). rows are already row-gated by the
// caller. A row whose body is inaccessible (git-crypt) is absent, so its
// roll-ups have no value: a count derived from the body is shown only where
// the body is. Field redaction does not reach the body today (see the
// body-redaction TODO on [PolicyReader]); when it does, this read must
// apply it too.
func readBodies(ctx context.Context, st store.EntityLister, rows []entity.Ref) (map[entity.Ref]string, error) {
	byFace := map[entity.Face][]string{}
	var faces []entity.Face
	seen := make(map[entity.Ref]struct{}, len(rows))
	for _, ref := range rows {
		if _, dup := seen[ref]; dup {
			continue
		}
		seen[ref] = struct{}{}
		if _, known := byFace[ref.Face]; !known {
			faces = append(faces, ref.Face)
		}
		byFace[ref.Face] = append(byFace[ref.Face], ref.ID)
	}
	out := make(map[entity.Ref]string, len(rows))
	for _, face := range faces {
		q := store.EntityQuery{IDs: byFace[face], Faces: store.AtFaces(face)}
		for e, err := range st.ListEntities(ctx, q) {
			if err != nil {
				return nil, err
			}
			if !bodyInaccessible(e) {
				out[e.Ref()] = e.Content
			}
		}
	}
	return out, nil
}

// readBodiesOrNone is [readBodies], or no bodies when the read fails: the
// body roll-ups are then absent, never ungated.
func readBodiesOrNone(
	ctx context.Context, st store.EntityLister, entityType string, rows []entity.Ref,
) map[entity.Ref]string {
	bodies, err := readBodies(ctx, st, rows)
	if err != nil {
		slog.Warn("visibility: body roll-ups left out", "type", entityType, "err", err)
		return map[entity.Ref]string{}
	}
	return bodies
}

// bodyColumn is one body_tasks roll-up over the readable bodies, keyed by
// id; the first face of an id wins.
func bodyColumn(col rollup.Column, rows []entity.Ref, bodies map[entity.Ref]string) map[string]store.RollupResult {
	out := make(map[string]store.RollupResult, len(rows))
	for _, ref := range rows {
		if _, done := out[ref.ID]; done {
			continue
		}
		if body, ok := bodies[ref]; ok {
			out[ref.ID] = store.RollupResult{Value: col.TaskCount(body)}
		}
	}
	return out
}

// bodyInaccessible reports whether e's body is marked unreadable.
func bodyInaccessible(e *entity.Entity) bool {
	for _, f := range e.Inaccessible {
		if f.Name == entity.InaccessibleFieldContent {
			return true
		}
	}
	return false
}
