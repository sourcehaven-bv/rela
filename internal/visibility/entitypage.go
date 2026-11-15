package visibility

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// PageQuery asks for one page of the rows of a type that a caller may read
// (TKT-D9UO9Y4). Rows come in ascending id order, then face order, the
// default order of [store.GraphQuery].
type PageQuery struct {
	// Type is the entity type. Required: the read scope is composed per
	// type.
	Type string
	// Faces is the caller's face selection, as on [store.EntityQuery].
	Faces store.FaceSelection
	// Props narrows the rows by the type's own properties. Each property
	// must be visible to the caller on every row it reads (see
	// [ErrPropsNotPushable]); the store compares raw values, so a property
	// hidden on some row would answer for a value the caller cannot see.
	Props []store.PropPredicate
	// Limit is the page size. It must be positive: a page read never
	// returns the whole type.
	Limit int
	// Offset counts rows in the ordered result.
	Offset int
}

// EntityPage is one page of a [PageQuery]. Rows carry no body: they are
// read as headers, then redacted for the caller.
type EntityPage struct {
	Rows []*entity.Entity
	// Total is how many rows the query matches for the caller: counted in
	// the store over the same read scope as Rows, never the type's raw row
	// count, which would tell the caller how many rows are hidden from it.
	Total int
}

// ErrPropsNotPushable reports a [PageQuery] whose Props name a property the
// caller does not see on every row. The caller answers such a filter over
// redacted rows instead.
var ErrPropsNotPushable = errors.New("visibility: a filtered property is hidden on some rows")

// errPageLimit reports a [PageQuery] without a positive Limit.
var errPageLimit = errors.New("visibility: a page query needs a positive limit")

// fieldVisibility is the redactor capability a filtered page read needs:
// whether the caller sees property on every row of entityType it reads.
// [PolicyRedactor] and [NopRedactor] implement it. A redactor without it
// gets no filter pushed, which is a refusal, never a weaker gate.
type fieldVisibility interface {
	FieldVisibleOnEveryRow(ctx context.Context, entityType, property string) bool
}

// FieldVisibleOnEveryRow implements [fieldVisibility]. See
// [affordances.PolicyResolver.FieldVisibleOnEveryRow].
func (p PolicyRedactor) FieldVisibleOnEveryRow(ctx context.Context, entityType, property string) bool {
	return p.r.FieldVisibleOnEveryRow(ctx, entityType, property)
}

// FieldVisibleOnEveryRow implements [fieldVisibility]: nothing is hidden.
func (NopRedactor) FieldVisibleOnEveryRow(context.Context, string, string) bool { return true }

// propsFilterable reports whether every property of props is visible to the
// caller on every row of entityType, so the store may filter on it.
func (r *PolicyReader) propsFilterable(ctx context.Context, entityType string, props []store.PropPredicate) bool {
	if len(props) == 0 {
		return true
	}
	fv, ok := r.redact.(fieldVisibility)
	if !ok {
		return false
	}
	for _, p := range props {
		if !fv.FieldVisibleOnEveryRow(ctx, entityType, p.Property) {
			return false
		}
	}
	return true
}

// propsFilterer is the [PolicyReader] half of a filtered page read, taken by
// type assertion like [pushdownRedactor].
type propsFilterer interface {
	propsFilterable(ctx context.Context, entityType string, props []store.PropPredicate) bool
}

// checkPageQuery rejects the query shapes every implementation refuses.
func checkPageQuery(q PageQuery) error {
	if q.Type == "" {
		return errors.New("visibility: a page query needs a type")
	}
	if q.Limit <= 0 {
		return errPageLimit
	}
	return nil
}

// ListEntityPage reads one page of the rows of q.Type the caller may read,
// redacted, and the number of rows the query matches for the caller.
//
// The row gate runs in the store: the page and the total are one
// [store.GraphQuery] and one [store.CountMatched] over the caller's composed
// read scope, so the cost follows the page, not the type. The field gate
// runs on the page, primed once for it (PolicyReader.redactPage).
//
// Props are pushed only when the caller sees each property on every row it
// reads; otherwise the read returns [ErrPropsNotPushable]. Without pushdown
// (a reader that cannot redact without gating, or no read-scope provider) a
// query without Props is answered by load-then-Filter, and one with Props
// is refused with ErrPropsNotPushable. Both are a cost or a refusal, never a
// weaker gate.
func (s *ScriptReader) ListEntityPage(ctx context.Context, q PageQuery) (EntityPage, error) {
	if err := checkPageQuery(q); err != nil {
		return EntityPage{}, err
	}
	bound := s.bind(ctx)
	if len(q.Props) > 0 {
		if pf, ok := s.reader.(propsFilterer); !ok || !pf.propsFilterable(bound, q.Type, q.Props) {
			return EntityPage{}, ErrPropsNotPushable
		}
	}
	red, ok := s.reader.(pushdownRedactor)
	if !ok {
		return s.pageViaFilter(bound, q)
	}
	scope, ok, err := composeReadScope(bound, s.provider, s.raw, store.EntityQuery{Type: q.Type, Faces: q.Faces})
	switch {
	case !ok:
		return s.pageViaFilter(bound, q)
	case err != nil:
		return EntityPage{}, err
	case scope.deny:
		return EntityPage{}, nil
	}
	var gq store.GraphQuery
	switch {
	case scope.allow != nil:
		gq = store.GraphQuery{EntityType: q.Type, Faces: q.Faces, FaceIn: scope.allow.FaceIn}
	case scope.graph != nil:
		gq = *scope.graph // composeReadScope already copied the ACL's query
	default:
		return EntityPage{}, errEmptyScope
	}
	raw, ok := s.raw.(store.GraphQueryer)
	if !ok { // coverage-ignore: composeReadScope returned ok, so raw is a GraphQueryer
		return EntityPage{}, errEmptyScope
	}
	return readPage(bound, raw, gq, q, red.redactPage)
}

// readPage runs the page and the count of q on gq, a copy of the caller's
// read scope, and redacts the page with redact.
func readPage(
	ctx context.Context, raw store.GraphQueryer, gq store.GraphQuery, q PageQuery, redact pageRedactor,
) (EntityPage, error) {
	gq.Props = slices.Concat(gq.Props, q.Props)
	total, err := store.CountMatched(ctx, raw, gq)
	if err != nil {
		return EntityPage{}, err
	}
	if total <= q.Offset {
		return EntityPage{Total: total}, nil
	}
	gq.Limit = q.Limit
	gq.Offset = max(q.Offset, 0)
	rows := make([]*entity.Entity, 0, min(q.Limit, total-q.Offset))
	for h, err := range store.GraphQueryHeaders(ctx, raw, gq) {
		if err != nil {
			return EntityPage{}, err
		}
		rows = append(rows, headerRow(h))
	}
	return EntityPage{Rows: redact(ctx, rows), Total: total}, nil
}

// headerRow is the content-free entity a page serves for header h.
func headerRow(h store.EntityHeader) *entity.Entity {
	return &entity.Entity{
		ID: h.ID, Type: h.Type, Face: h.Face, Properties: h.Properties,
		UpdatedAt: h.UpdatedAt, Inaccessible: h.Inaccessible,
	}
}

// pageViaFilter is [ScriptReader.ListEntityPage] without pushdown: the rows
// of the type, gated by Filter, then cut to the page. It reads the whole
// type, so it only serves a reader that cannot push the gate down; no
// production wiring builds one.
func (s *ScriptReader) pageViaFilter(ctx context.Context, q PageQuery) (EntityPage, error) {
	if len(q.Props) > 0 {
		return EntityPage{}, ErrPropsNotPushable
	}
	rows, err := s.listGatedThenRanked(ctx, store.EntityQuery{Type: q.Type, Faces: q.Faces})
	if err != nil {
		return EntityPage{}, err
	}
	kept := s.reader.Filter(ctx, rows)
	slices.SortFunc(kept, compareRowKey)
	total := len(kept)
	start := min(max(q.Offset, 0), total)
	page := kept[start:min(start+q.Limit, total)]
	for i, e := range page {
		c := *e
		c.Content = ""
		page[i] = &c
	}
	return EntityPage{Rows: page, Total: total}, nil
}

// compareRowKey orders rows as [store.GraphQuery] does by default: id, then
// face.
func compareRowKey(a, b *entity.Entity) int {
	return cmp.Or(cmp.Compare(a.ID, b.ID), cmp.Compare(a.Face, b.Face))
}

// ListEntityPage implements the page read with no gate: one
// [store.GraphQuery] page and one [store.CountMatched] on the raw store.
func (r *UnrestrictedReader) ListEntityPage(ctx context.Context, q PageQuery) (EntityPage, error) {
	if err := checkPageQuery(q); err != nil {
		return EntityPage{}, err
	}
	gq, ok := r.st.(store.GraphQueryer)
	if !ok { // coverage-ignore: store.Store embeds GraphQueryer
		return EntityPage{}, fmt.Errorf("visibility: store %T cannot run a graph query", r.st)
	}
	keep := func(_ context.Context, rows []*entity.Entity) []*entity.Entity { return rows }
	return readPage(ctx, gq, store.GraphQuery{EntityType: q.Type, Faces: q.Faces}, q, keep)
}

// ListEntityPage implements the page read: always refuses.
func (DenyReader) ListEntityPage(context.Context, PageQuery) (EntityPage, error) {
	return EntityPage{}, ErrReaderUnavailable
}
