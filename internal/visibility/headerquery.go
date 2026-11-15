package visibility

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// HeaderQuery names the rows [HolderLookup.Headers] reads: the rows of
// EntityType where every Equal property holds its value and, when AnyOf is
// set, at least one AnyOf group holds in full. Values compare as scalar
// strings.
type HeaderQuery struct {
	EntityType string
	Equal      map[string]string
	AnyOf      []map[string]string
	Faces      store.FaceSelection
	// Limit is the most rows returned; it must be positive.
	Limit int
	// Order, when set, picks which rows the Limit keeps and the order they
	// are returned in, with [store.GraphQuery.OrderBy] semantics: the
	// string form compared byte-wise, a row without the property last
	// ascending, then id. Empty returns the rows in (id, face) order.
	Order []store.OrderSpec
}

// Headers returns the headers of the rows q names that the caller may
// read, redacted, at most q.Limit of them; more reports that at least one
// more matched (RR-UPVFJIR). Like [HolderLookup.Matching] it composes the
// caller's read scope with the caller's predicates as one store query: the
// equalities are ANDed onto every verdict, and AnyOf rides on
// [store.GraphQuery.Narrowing], which only narrows. The predicates are
// tested again on the redacted row, so a value `visible:` hides from the
// caller never selects a row.
func (h *HolderLookup) Headers(ctx context.Context, q HeaderQuery) (rows []store.EntityHeader, more bool,
	err error) {
	if q.EntityType == "" || q.Limit <= 0 {
		return nil, false, fmt.Errorf("%w: header query needs a type and a positive limit", store.ErrInvalidQuery)
	}
	if verr := q.Faces.Validate(); verr != nil {
		return nil, false, verr
	}
	if h.refuse {
		return nil, false, ErrReaderUnavailable
	}
	keep := func(hd store.EntityHeader) store.EntityHeader { return hd }
	if h.sr == nil {
		base := headerBase(store.GraphQuery{EntityType: q.EntityType, Faces: q.Faces}, q)
		return pageHeaders(ctx, h.raw, base, q, keep)
	}
	bound := h.sr.bind(ctx)
	gq, isGQ := h.sr.raw.(store.GraphQueryer)
	red, isRed := h.sr.reader.(headerRedactor)
	if h.sr.provider == nil || !isGQ || !isRed {
		listed, lerr := h.sr.listGatedThenRanked(bound, store.EntityQuery{Type: q.EntityType, Faces: q.Faces})
		if lerr != nil {
			return nil, false, lerr
		}
		var out []store.EntityHeader
		for _, e := range h.sr.reader.Filter(bound, listed) {
			hd := store.EntityHeader{ID: e.ID, Type: e.Type, Face: e.Face, Properties: e.Properties}
			if headerMatches(hd, q) {
				out = append(out, hd)
			}
		}
		return cutHeaders(out, q)
	}
	rqr, err := h.sr.provider.ReadQueryFor(bound, q.EntityType)
	if err != nil {
		return nil, false, err
	}
	var base store.GraphQuery
	switch {
	case rqr.DenyAll:
		return nil, false, nil
	case rqr.AllowAll:
		base = store.GraphQuery{EntityType: q.EntityType, Faces: q.Faces}
	case rqr.Query != nil:
		// Copy before stamping: the ACL layer may reuse its query.
		base = *rqr.Query
		base.Props = slices.Clone(base.Props)
		base.Narrowing = slices.Clone(base.Narrowing)
	default:
		return nil, false, errZeroReadQuery
	}
	base = headerBase(base, q)
	base.FaceIn = rqr.Faces
	redact := func(hd store.EntityHeader) store.EntityHeader { return red.RedactHeaderRow(bound, hd) }
	return pageHeaders(bound, gq, base, q, redact)
}

// headerBase stamps q's predicates and face selection onto base.
func headerBase(base store.GraphQuery, q HeaderQuery) store.GraphQuery {
	for _, k := range sortedKeys(q.Equal) {
		base.Props = append(base.Props, store.PropPredicate{Property: k, Op: store.PropEqual, Value: q.Equal[k],
			Scalar: true})
	}
	for _, g := range q.AnyOf {
		var br store.NarrowBranch
		for _, k := range sortedKeys(g) {
			br.Props = append(br.Props, store.PropPredicate{Property: k, Op: store.PropEqual, Value: g[k],
				Scalar: true})
		}
		base.Narrowing = append(base.Narrowing, br)
	}
	base.Faces = q.Faces
	return base
}

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}

// pageHeaders runs base in bounded pages of q.Limit+1 header rows until
// q.Limit+1 rows survive redaction and the predicates, or the rows run out.
func pageHeaders(ctx context.Context, gq store.GraphQueryer, base store.GraphQuery, q HeaderQuery,
	redact func(store.EntityHeader) store.EntityHeader) ([]store.EntityHeader, bool, error) {
	base.Limit = q.Limit + 1
	base.OrderBy = q.Order
	var out []store.EntityHeader
	for offset := 0; ; offset += base.Limit {
		base.Offset = offset
		read := 0
		for hd, err := range store.GraphQueryHeaders(ctx, gq, base) {
			if err != nil {
				return nil, false, err
			}
			read++
			if r := redact(hd); headerMatches(r, q) {
				out = append(out, r)
			}
		}
		if len(out) > q.Limit || read < base.Limit {
			break
		}
	}
	return cutHeaders(out, q)
}

// headerMatches tests q's predicates on a (redacted) row in Go.
func headerMatches(hd store.EntityHeader, q HeaderQuery) bool {
	holds := func(g map[string]string) bool {
		for k, v := range g {
			if s, ok := hd.Properties[k].(string); !ok || s != v {
				return false
			}
		}
		return true
	}
	if !holds(q.Equal) {
		return false
	}
	return len(q.AnyOf) == 0 || slices.ContainsFunc(q.AnyOf, holds)
}

// cutHeaders sorts rows in q.Order, then (id, face) order, and cuts them
// to q.Limit.
func cutHeaders(rows []store.EntityHeader, q HeaderQuery) ([]store.EntityHeader, bool, error) {
	slices.SortFunc(rows, func(a, b store.EntityHeader) int {
		for _, o := range q.Order {
			if c := compareOrderValue(a.Properties[o.Property], b.Properties[o.Property]); c != 0 {
				if o.Descending {
					return -c
				}
				return c
			}
		}
		return store.CompareRefs(entity.Ref{ID: a.ID, Face: a.Face}, entity.Ref{ID: b.ID, Face: b.Face})
	})
	if len(rows) > q.Limit {
		return rows[:q.Limit], true, nil
	}
	return rows, false, nil
}

// compareOrderValue compares two property values by their string form,
// byte-wise; a missing value sorts after every present one.
func compareOrderValue(a, b any) int {
	switch {
	case a == nil && b == nil:
		return 0
	case a == nil:
		return 1
	case b == nil:
		return -1
	}
	return strings.Compare(orderString(a), orderString(b))
}

// orderString is the string form of v that [store.GraphQuery.OrderBy]
// sorts by.
func orderString(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case time.Time:
		return t.UTC().Format(time.RFC3339Nano)
	default:
		return fmt.Sprint(v)
	}
}
