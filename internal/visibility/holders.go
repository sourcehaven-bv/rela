package visibility

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// errZeroReadQuery reports a read scope with no verdict and no query. It is
// an error here, not a fallback: a holder query must not guess.
var errZeroReadQuery = errors.New("visibility: read scope has neither a verdict nor a query")

// HolderQuery names the rows [HolderLookup.Matching] finds: the rows of
// EntityType whose Property names Identity.
type HolderQuery struct {
	EntityType string
	Property   string
	// Identity is the value the property must hold: the string itself, or
	// an element of a list. "" matches no row, never an unset property.
	Identity string
	// Faces is the result selection, as on a list read. Under a world, an
	// id matches only when the face the world serves the caller names
	// Identity.
	Faces store.FaceSelection
	// Limit is the most refs returned; it must be positive.
	Limit int
}

// HolderLookup finds the rows of one type whose property names an
// identity, through the same gate, redactor, store and world as the script
// readers. The limit counts rows after the row gate AND after field
// redaction: a row whose property `visible:` hides from the caller is
// neither returned nor counted.
//
// It is its own type rather than a reader method because the script
// readers are at the plimsoll method cap and only realtime asks this
// question (TKT-ZZ4PNX).
type HolderLookup struct {
	// sr is the gated reader; nil is the unrestricted or refusing mode.
	sr *ScriptReader
	// raw serves the unrestricted mode.
	raw store.Store
	// refuse is the refusing mode.
	refuse bool
}

// NewHolderLookup builds the gated lookup over sr: sr's gate composes the
// row scope, its reader redacts each row, and its store runs the query.
// Nil: rejected — a lookup without a gate would read ungated.
func NewHolderLookup(sr *ScriptReader) (*HolderLookup, error) {
	if sr == nil {
		return nil, errors.New("visibility: NewHolderLookup: script reader must be non-nil")
	}
	return &HolderLookup{sr: sr}, nil
}

// NewUnrestrictedHolderLookup builds the lookup for the NopACL path: no row
// gate and no redaction, like [UnrestrictedReader].
// Nil: rejected — there is nothing to read.
func NewUnrestrictedHolderLookup(st store.Store) (*HolderLookup, error) {
	if st == nil {
		return nil, errors.New("visibility: NewUnrestrictedHolderLookup: store must be non-nil")
	}
	return &HolderLookup{raw: st}, nil
}

// NewRefusingHolderLookup builds the lookup for a wiring whose gate failed:
// every call is [ErrReaderUnavailable], as [DenyReader] answers.
func NewRefusingHolderLookup() *HolderLookup { return &HolderLookup{refuse: true} }

// Matching returns the refs of the rows q names that the caller may read,
// in (id, face) order, at most q.Limit of them; more reports that at least
// one more matched.
//
// Gated, the caller's read scope for the type is composed as a store query
// and the property test is ANDed onto every verdict, AllowAll included, so
// the row gate, the face allowlist and the world apply as on a list read.
// Each row is redacted and the property is tested again on the redacted
// row. A store without graph queries, or a reader that cannot redact a row
// on its own, gets the same answer from a gated list read filtered in Go.
func (h *HolderLookup) Matching(ctx context.Context, q HolderQuery) (refs []entity.Ref, more bool, err error) {
	if q.EntityType == "" || q.Property == "" || q.Limit <= 0 {
		return nil, false, fmt.Errorf("%w: holder query needs a type, a property and a positive limit",
			store.ErrInvalidQuery)
	}
	if err := q.Faces.Validate(); err != nil {
		return nil, false, err
	}
	switch {
	case h.refuse:
		return nil, false, ErrReaderUnavailable
	case q.Identity == "":
		return nil, false, nil
	case h.sr == nil:
		return h.unrestricted(ctx, q)
	}
	return h.gated(ctx, q)
}

// gated is Matching through the script reader's gate and redactor.
func (h *HolderLookup) gated(ctx context.Context, q HolderQuery) ([]entity.Ref, bool, error) {
	bound := h.sr.bind(ctx)
	gq, isGQ := h.sr.raw.(store.GraphQueryer)
	red, isRed := h.sr.reader.(headerRedactor)
	if h.sr.provider == nil || !isGQ || !isRed {
		return h.gatedFallback(bound, q)
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
	default:
		return nil, false, errZeroReadQuery
	}
	// Under a world the store tests Props on the face it serves, not on
	// a lower face (store.GraphQuery), so q.Faces passes through as on a
	// list read.
	base.Props = append(base.Props, holderProp(q))
	base.FaceIn = rqr.Faces
	base.Faces = q.Faces
	redact := func(hd store.EntityHeader) store.EntityHeader { return red.RedactHeaderRow(bound, hd) }
	return pageHolders(bound, gq, base, q, redact)
}

// gatedFallback answers a gated query without pushdown: a gated list read
// of the type, redacted, filtered in Go.
func (h *HolderLookup) gatedFallback(ctx context.Context, q HolderQuery) ([]entity.Ref, bool, error) {
	rows, err := h.sr.listGatedThenRanked(ctx, store.EntityQuery{Type: q.EntityType, Faces: q.Faces})
	if err != nil {
		return nil, false, err
	}
	return firstHolders(h.sr.reader.Filter(ctx, rows), q)
}

// unrestricted is Matching with no gate and no redaction.
func (h *HolderLookup) unrestricted(ctx context.Context, q HolderQuery) ([]entity.Ref, bool, error) {
	gq, ok := h.raw.(store.GraphQueryer)
	if !ok {
		var rows []*entity.Entity
		for e, err := range h.raw.ListEntities(ctx, store.EntityQuery{Type: q.EntityType, Faces: q.Faces}) {
			if err != nil {
				return nil, false, err
			}
			rows = append(rows, e)
		}
		return firstHolders(rows, q)
	}
	base := store.GraphQuery{EntityType: q.EntityType, Props: []store.PropPredicate{holderProp(q)}, Faces: q.Faces}
	keep := func(hd store.EntityHeader) store.EntityHeader { return hd }
	return pageHolders(ctx, gq, base, q, keep)
}

// holderProp is the store predicate of q: non-scalar equality, which is
// equality on a scalar value and membership on a list value, whatever the
// schema now says about the property's shape.
func holderProp(q HolderQuery) store.PropPredicate {
	return store.PropPredicate{Property: q.Property, Op: store.PropEqual, Value: q.Identity, Scalar: false}
}

// pageHolders runs base as a content-free query in bounded pages of
// q.Limit+1 rows, ordered by the backend, until q.Limit+1 rows survive
// redaction and the property test, or the rows run out. A page whose rows
// all fail the test costs one more page, never an unbounded read. Only
// headers are read: the lookup needs the id, the face and one property,
// never a body.
func pageHolders(
	ctx context.Context, gq store.GraphQueryer, base store.GraphQuery, q HolderQuery,
	redact func(store.EntityHeader) store.EntityHeader,
) ([]entity.Ref, bool, error) {
	base.Limit = q.Limit + 1
	var out []entity.Ref
	for offset := 0; ; offset += base.Limit {
		base.Offset = offset
		read := 0
		for hd, err := range store.GraphQueryHeaders(ctx, gq, base) {
			if err != nil {
				return nil, false, err
			}
			read++
			if r := redact(hd); namesIdentity(r.Properties[q.Property], q.Identity) {
				out = append(out, entity.Ref{ID: r.ID, Face: r.Face})
			}
		}
		if len(out) > q.Limit || read < base.Limit {
			break
		}
	}
	return truncateRefs(out, q.Limit)
}

// firstHolders filters rows in Go and returns the first q.Limit in (id,
// face) order.
func firstHolders(rows []*entity.Entity, q HolderQuery) ([]entity.Ref, bool, error) {
	var out []entity.Ref
	for _, e := range rows {
		if e != nil && namesIdentity(e.Properties[q.Property], q.Identity) {
			out = append(out, e.Ref())
		}
	}
	return truncateRefs(out, q.Limit)
}

// truncateRefs sorts refs in (id, face) order and cuts them to limit.
func truncateRefs(refs []entity.Ref, limit int) ([]entity.Ref, bool, error) {
	slices.SortFunc(refs, store.CompareRefs)
	refs = slices.CompactFunc(refs, func(a, b entity.Ref) bool { return a == b })
	if len(refs) > limit {
		return refs[:limit], true, nil
	}
	return refs, false, nil
}

// namesIdentity reports whether v names identity: v is the string, or a
// list with the string as an element. It is the rule is_current_user and
// has_current_user apply; predicatefns is out of this package's reach, so
// the byte equality is written here.
func namesIdentity(v any, identity string) bool {
	switch x := v.(type) {
	case string:
		return x == identity
	case []string:
		return slices.Contains(x, identity)
	case []any:
		for _, el := range x {
			if s, ok := el.(string); ok && s == identity {
				return true
			}
		}
	}
	return false
}
