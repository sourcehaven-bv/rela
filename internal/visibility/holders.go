package visibility

import (
	"context"
	"errors"
	"fmt"
	"maps"
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
// readers are at the plimsoll method cap. Realtime asks it for holders
// (TKT-ZZ4PNX) and ingest for its find ([HolderLookup.FirstEqual]).
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

// HolderLookupFor returns the lookup on the tier of r, a reader the wiring
// built: gated over a [ScriptReader], unrestricted over an
// [UnrestrictedReader]'s store, and refusing for a [DenyReader] or any other
// reader, so a reader this function does not know fails closed rather than
// reading ungated.
func HolderLookupFor(r store.EntityLister) *HolderLookup {
	switch x := r.(type) {
	case *ScriptReader:
		if x != nil {
			return &HolderLookup{sr: x}
		}
	case *UnrestrictedReader:
		if x != nil && x.st != nil {
			return &HolderLookup{raw: x.st}
		}
	}
	return NewRefusingHolderLookup()
}

// Matching returns the refs of the rows q names that the caller may read,
// in (id, face) order, at most q.Limit of them; more reports that at least
// one more matched.
//
// Gated, the caller's read scope for the type is composed as a store query
// and the property test is ANDed onto every verdict, AllowAll included, so
// the row gate, the face allowlist and the world apply as on a list read.
// Each row is redacted and the property is tested again on the redacted
// row. A reader with no read-query provider, or one that cannot redact a
// row on its own, gets the same answer from a gated list read filtered in
// Go.
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
	}
	return h.scan(ctx, holderScan{
		entityType: q.EntityType,
		faces:      q.Faces,
		props:      []store.PropPredicate{holderProp(q)},
		match:      func(row map[string]any) bool { return namesIdentity(row[q.Property], q.Identity) },
		limit:      q.Limit,
	})
}

// FirstEqual returns the readable row of entityType, at faces, with the
// lowest (id, face) whose every property in want holds its value; found is
// false when no readable row does. It is the find of webhook and feed
// ingest (TKT-GIAGLQ3).
//
// A property holds a value when it is a string equal to it, or a scalar
// whose fmt form equals it (a number, a bool). A list, an object or an
// unset property holds no value, and an empty value in want matches no
// row. The rows pass the same gate, face allowlist, world and redaction as
// [HolderLookup.Matching], and the test runs on the redacted row, so a
// property the caller may not see holds nothing.
func (h *HolderLookup) FirstEqual(
	ctx context.Context, entityType string, faces store.FaceSelection, want map[string]string,
) (ref entity.Ref, found bool, err error) {
	if entityType == "" || len(want) == 0 {
		return entity.Ref{}, false, fmt.Errorf("%w: equality lookup needs a type and a property",
			store.ErrInvalidQuery)
	}
	if vErr := faces.Validate(); vErr != nil {
		return entity.Ref{}, false, vErr
	}
	if h.refuse {
		return entity.Ref{}, false, ErrReaderUnavailable
	}
	props := make([]store.PropPredicate, 0, len(want))
	for _, prop := range slices.Sorted(maps.Keys(want)) {
		if want[prop] == "" {
			return entity.Ref{}, false, nil
		}
		// Non-scalar equality, so a number or bool matches its text form
		// in the store as in Go. It also admits a list holding the value,
		// which the match below drops.
		props = append(props, store.PropPredicate{Property: prop, Op: store.PropEqual, Value: want[prop]})
	}
	refs, _, err := h.scan(ctx, holderScan{
		entityType: entityType,
		faces:      faces,
		props:      props,
		match: func(row map[string]any) bool {
			for prop, value := range want {
				if !holdsScalar(row[prop], value) {
					return false
				}
			}
			return true
		},
		limit: 1,
		// Rows the store admits and the match drops (a list holding the
		// value, a property hidden from the caller) cost a page each; a
		// wider page bounds the round trips they take.
		page: firstEqualPage,
	})
	if err != nil || len(refs) == 0 {
		return entity.Ref{}, false, err
	}
	return refs[0], true, nil
}

// firstEqualPage is the page size of [HolderLookup.FirstEqual].
const firstEqualPage = 64

// holderScan is one lookup of [HolderLookup]: the rows of entityType at
// faces that props select in the store and match accepts on the redacted
// row, at most limit of them, read page rows at a time (at least limit+1).
type holderScan struct {
	entityType string
	faces      store.FaceSelection
	props      []store.PropPredicate
	match      func(row map[string]any) bool
	limit      int
	page       int
}

// scan runs q on the gated or the unrestricted mode.
func (h *HolderLookup) scan(ctx context.Context, q holderScan) ([]entity.Ref, bool, error) {
	if h.sr == nil {
		return h.unrestricted(ctx, q)
	}
	return h.gated(ctx, q)
}

// gated is a scan through the script reader's gate and redactor.
func (h *HolderLookup) gated(ctx context.Context, q holderScan) ([]entity.Ref, bool, error) {
	bound := h.sr.bind(ctx)
	gq, isGQ := h.sr.raw.(store.GraphQueryer)
	red, isRed := h.sr.reader.(headerRedactor)
	if h.sr.provider == nil || !isGQ || !isRed {
		return h.gatedFallback(bound, q)
	}
	rqr, err := h.sr.provider.ReadQueryFor(bound, q.entityType)
	if err != nil {
		return nil, false, err
	}
	var base store.GraphQuery
	switch {
	case rqr.DenyAll:
		return nil, false, nil
	case rqr.AllowAll:
		base = store.GraphQuery{EntityType: q.entityType, Faces: q.faces}
	case rqr.Query != nil:
		// Copy before stamping: the ACL layer may reuse its query.
		base = *rqr.Query
		base.Props = slices.Clone(base.Props)
	default:
		return nil, false, errZeroReadQuery
	}
	// Under a world the store tests Props on the face it serves, not on
	// a lower face (store.GraphQuery), so q.faces passes through as on a
	// list read.
	base.Props = append(base.Props, q.props...)
	base.FaceIn = rqr.Faces
	base.Faces = q.faces
	redact := func(hd store.EntityHeader) store.EntityHeader { return red.RedactHeaderRow(bound, hd) }
	return pageHolders(bound, gq, base, q, redact)
}

// gatedFallback answers a gated scan without pushdown: a gated list read
// of the type, redacted, filtered in Go.
func (h *HolderLookup) gatedFallback(ctx context.Context, q holderScan) ([]entity.Ref, bool, error) {
	rows, err := h.sr.listGatedThenRanked(ctx, store.EntityQuery{Type: q.entityType, Faces: q.faces})
	if err != nil {
		return nil, false, err
	}
	return firstHolders(h.sr.reader.Filter(ctx, rows), q)
}

// unrestricted is a scan with no gate and no redaction.
func (h *HolderLookup) unrestricted(ctx context.Context, q holderScan) ([]entity.Ref, bool, error) {
	base := store.GraphQuery{EntityType: q.entityType, Props: slices.Clone(q.props), Faces: q.faces}
	keep := func(hd store.EntityHeader) store.EntityHeader { return hd }
	return pageHolders(ctx, h.raw, base, q, keep)
}

// holderProp is the store predicate of q: non-scalar equality, which is
// equality on a scalar value and membership on a list value, whatever the
// schema now says about the property's shape.
func holderProp(q HolderQuery) store.PropPredicate {
	return store.PropPredicate{Property: q.Property, Op: store.PropEqual, Value: q.Identity, Scalar: false}
}

// pageHolders runs base as a content-free query in bounded pages of
// q.limit+1 rows (or q.page, when larger), ordered by the backend, until q.limit+1 rows survive
// redaction and the match, or the rows run out. A page whose rows all
// fail the match costs one more page, never an unbounded read. Only
// headers are read: the lookup needs the id, the face and the tested
// properties, never a body.
func pageHolders(
	ctx context.Context, gq store.GraphQueryer, base store.GraphQuery, q holderScan,
	redact func(store.EntityHeader) store.EntityHeader,
) ([]entity.Ref, bool, error) {
	base.Limit = max(q.limit+1, q.page)
	var out []entity.Ref
	for offset := 0; ; offset += base.Limit {
		base.Offset = offset
		read := 0
		for hd, err := range store.GraphQueryHeaders(ctx, gq, base) {
			if err != nil {
				return nil, false, err
			}
			read++
			if r := redact(hd); q.match(r.Properties) {
				out = append(out, entity.Ref{ID: r.ID, Face: r.Face})
			}
		}
		if len(out) > q.limit || read < base.Limit {
			break
		}
	}
	return truncateRefs(out, q.limit)
}

// firstHolders filters rows in Go and returns the first q.limit in (id,
// face) order.
func firstHolders(rows []*entity.Entity, q holderScan) ([]entity.Ref, bool, error) {
	var out []entity.Ref
	for _, e := range rows {
		if e != nil && q.match(e.Properties) {
			out = append(out, e.Ref())
		}
	}
	return truncateRefs(out, q.limit)
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

// holdsScalar reports whether v holds value as [HolderLookup.FirstEqual]
// defines it.
func holdsScalar(v any, value string) bool {
	switch x := v.(type) {
	case string:
		return x == value
	case nil, []any, []string, map[string]any, map[string]string:
		return false
	default:
		return fmt.Sprint(x) == value
	}
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
