package visibility

import (
	"context"
	"errors"
	"fmt"
	"iter"
	"log/slog"
	"maps"
	"slices"

	"github.com/Sourcehaven-BV/rela/internal/canonical"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/reconcile"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// Elements answers the element range read of rela-to-rela sync
// (FEAT-CMVZQFI) for the reader bound on ctx: the count and fingerprint sum
// of the elements in an address range, and the elements themselves, each
// with the reader's view hash (TKT-N1KNNZU).
//
// It completes the store's element range read (store.ElementScope). A type
// on which the reader has no hidden field and no roll-up is folded from its
// stored elements. Every row of any other type is loaded, and its view hash
// is computed in Go from the row as [Redact] returns it, with
// [reconcile.EntityElement], the function the ETag is built on. The store
// is never asked which of those rows hold a hidden value. The order is the gantt endpoint's: the store gates
// the rows, each row is redacted exactly once from a raw store row, and only
// then is it hashed and folded.
//
// # Scope
//
// The caller names the types in play ([ElementTypes]); every call compiles
// the store scope from the request bound on ctx, through the same policy
// collaborators the gated reads use. No scope outlives the call, so a grant
// or membership change applies on the next one. Like every gated read, a ctx
// without a bound request fails closed.
//
// A relation type whose meta the policy can hide ([ElementPolicy]) is
// refused with [ErrRelationMetaGated]: its view would need the meta
// redaction of the source row, which visibility does not do (TKT-0RBFN0).
//
// # What the caller can observe
//
// A row is pending in the store when a hidden property is set on it, which
// the reader must not learn. So the work never depends on it:
//
//   - A range in which every type folds stored rows (the reader has no
//     `visible:` restriction on any of them) is answered by one store
//     aggregate. Such a row is pending only when its stored element is not
//     filled yet.
//   - Any other range is listed once, and every row of a type that is not
//     fully folded is loaded, redacted and hashed in Go, pending or not.
//     Whether the call takes this path, and whether it is refused with
//     [ErrRangeTooLarge], depend on the scope and the row count only.
//
// The CPU time still varies a little with what [Redact] removes; that is
// the same exposure the ETag path has.
//
// # Cost and consistency
//
// A listed range costs one ListRange, and per type with rows to complete one
// ListEntities and at most one MatchingFaces (or one ListRelations), plus the
// priming queries of [primePage], whatever the number of rows. The rows of
// one call are capped ([ElementDeps.MaxRows]); each completed row's body is
// loaded and formatted, because the view hash covers the formatted body
// (RR-3HN68CI). The cap bounds rows, not bytes.
//
// Stored hashes are read when the range is listed, completed hashes when
// the rows are loaded. A loaded entity is checked against the row gate
// again and dropped, uncounted, when it fails it or is gone. A relation
// completed from its row (only when its stored element is not filled) keeps
// the endpoint verdict of the listing.
//
// The result is one reader's. Never cache it, or reuse it for another
// principal.
type Elements struct {
	ranges  ElementRangeReader
	rows    ElementRowReader
	reads   ElementReads
	policy  ElementPolicy
	rollups RollupNamer
	maxRows int
}

// ElementRangeReader is the store's element range reader: elementnaive for
// memstore and fsstore, pgstore.NewElementRanges and
// sqlitestore.NewElementRanges for the database backends.
type ElementRangeReader interface {
	RangeFingerprint(ctx context.Context, scope store.ElementScope, r store.ElementRange) (store.RangeSum, error)
	ListRange(
		ctx context.Context, scope store.ElementScope, r store.ElementRange, l store.RangeList,
	) ([]store.RangeEntry, error)
}

// ElementRowReader loads the raw rows an [Elements] call completes, and
// checks loaded entities against the row gate.
type ElementRowReader interface {
	ListEntities(ctx context.Context, q store.EntityQuery) iter.Seq2[*entity.Entity, error]
	ListRelations(ctx context.Context, q store.RelationQuery) iter.Seq2[*entity.Relation, error]
	MatchingFaces(ctx context.Context, q store.GraphQuery, ids []string) (map[string][]entity.Face, error)
}

// ElementReads is the row half of the read policy: the read query per type,
// and the check that ctx carries a bound request, which a call makes before
// anything else. [DeclarativeGate] and [AllowAllReadQueries] implement it.
type ElementReads interface {
	ReadQueryProvider
	Bind(ctx context.Context) (context.Context, error)
}

// ElementPolicy is the field half of the read policy, as [Elements] needs it.
// One value supplies all three answers, so the decision which rows fold a
// stored hash (from the field scope) and the view a completed row is hashed
// from (from the hidden properties) come from one policy. [PolicyRedactor]
// and [NopRedactor] implement it.
type ElementPolicy interface {
	FieldScoper
	FieldRedactor
	// RelationMetaGated reports whether the policy can hide a meta key of
	// some relation of relType from some reader. It is the same for every
	// principal.
	RelationMetaGated(relType string) bool
}

// RollupNamer names the roll-up properties of an entity type. [*Rollups]
// implements it. A nil *Rollups names none; [NewElements] accepts it and
// rejects only a nil interface.
type RollupNamer interface {
	Names(entityType string) []string
}

// ElementDeps are the collaborators of [Elements]. All but MaxRows are
// required.
type ElementDeps struct {
	Ranges  ElementRangeReader
	Rows    ElementRowReader
	Reads   ElementReads
	Policy  ElementPolicy
	Rollups RollupNamer
	// MaxRows caps the rows one listed call covers, 1 to
	// store.MaxRangeList-1; zero means [DefaultElementMaxRows].
	MaxRows int
}

// DefaultElementMaxRows is the row cap of a listed [Elements] call when
// [ElementDeps.MaxRows] is zero.
const DefaultElementMaxRows = 2000

// ErrRangeTooLarge refuses a range that would list more rows than the cap.
// The caller splits the range. The refusal depends on the scope and the row
// count. For a range of types folded from stored elements it also depends
// on whether some stored element is not filled yet (a database backend that
// is still backfilling). Neither is secret, but the same range can be
// refused during a backfill and answered after it.
var ErrRangeTooLarge = errors.New("visibility: element range has more rows than one call covers")

// ErrRelationMetaGated refuses a relation type whose meta the policy can
// hide. Its element would need the meta redaction of the source row.
var ErrRelationMetaGated = errors.New("visibility: relation meta visibility is not supported in element ranges")

// ElementTypes names the types an [Elements] call covers. A type not named
// has no rows in range.
type ElementTypes struct {
	Entities  []string
	Relations []string
}

// NewElements builds the element range read.
//
// Nil: every collaborator rejected (a nil interface; [RollupNamer] says
// which typed nil it admits), as a missing gate or policy would fold
// ungated rows or unredacted views.
func NewElements(d ElementDeps) (*Elements, error) {
	switch {
	case d.Ranges == nil:
		return nil, errors.New("visibility: NewElements: range reader is required")
	case d.Rows == nil:
		return nil, errors.New("visibility: NewElements: row reader is required")
	case d.Reads == nil:
		return nil, errors.New("visibility: NewElements: read query provider is required")
	case d.Policy == nil:
		return nil, errors.New("visibility: NewElements: element policy is required")
	case d.Rollups == nil:
		return nil, errors.New("visibility: NewElements: roll-up namer is required")
	}
	maxRows := d.MaxRows
	if maxRows == 0 {
		maxRows = DefaultElementMaxRows
	}
	if maxRows < 1 || maxRows >= store.MaxRangeList {
		return nil, fmt.Errorf("visibility: NewElements: MaxRows %d is outside 1..%d", maxRows, store.MaxRangeList-1)
	}
	return &Elements{
		ranges: d.Ranges, rows: d.Rows, reads: d.Reads, policy: d.Policy, rollups: d.Rollups, maxRows: maxRows,
	}, nil
}

// Fingerprint returns the count and fingerprint sum of the elements of types
// in r, as the reader bound on ctx sees them.
func (x *Elements) Fingerprint(ctx context.Context, types ElementTypes, r store.ElementRange) (reconcile.Range, error) {
	c, err := x.compile(ctx, types, r)
	if err != nil {
		return reconcile.Range{}, err
	}
	if c.plan.Empty() {
		return reconcile.Range{}, nil
	}
	if c.foldsStored() {
		sum, ferr := x.ranges.RangeFingerprint(ctx, c.scope, r)
		if ferr != nil {
			return reconcile.Range{}, fmt.Errorf("visibility: element range: fingerprint: %w", ferr)
		}
		if sum.Pending == 0 {
			return reconcile.Range{Count: sum.Count, Sum: reconcile.FingerprintFromLimbSums(sum.LimbSums)}, nil
		}
	}
	entries, err := x.ranges.ListRange(ctx, c.scope, r, store.RangeList{Limit: x.maxRows + 1})
	if err != nil {
		return reconcile.Range{}, fmt.Errorf("visibility: element range: list: %w", err)
	}
	if len(entries) > x.maxRows {
		return reconcile.Range{}, fmt.Errorf("%w: more than %d", ErrRangeTooLarge, x.maxRows)
	}
	els, err := x.complete(ctx, c, entries)
	if err != nil {
		return reconcile.Range{}, err
	}
	var out reconcile.Range
	for _, e := range els {
		out = out.Add(e)
	}
	return out, nil
}

// ElementPage is one [Elements.List] answer.
//
// Elements holds the elements in address order. A row that is gone or no
// longer readable when it is loaded is left out, so Elements may be shorter
// than the limit, or empty, while the range has more rows. Last is the
// address of the last row listed, kept or not, and More reports whether the
// listing stopped at the limit. The caller resumes after Last, never after
// the last element it got.
type ElementPage struct {
	Elements []reconcile.Element
	Last     string
	More     bool
}

// List returns the first limit rows of types in r in address order as
// elements, as the reader bound on ctx sees them. limit is 1 to the row cap.
func (x *Elements) List(
	ctx context.Context, types ElementTypes, r store.ElementRange, limit int,
) (ElementPage, error) {
	if limit < 1 || limit > x.maxRows {
		return ElementPage{}, fmt.Errorf("%w: element list: limit %d is outside 1..%d",
			store.ErrInvalidQuery, limit, x.maxRows)
	}
	c, err := x.compile(ctx, types, r)
	if err != nil {
		return ElementPage{}, err
	}
	if c.plan.Empty() {
		return ElementPage{}, nil
	}
	entries, err := x.ranges.ListRange(ctx, c.scope, r, store.RangeList{Limit: limit})
	if err != nil {
		return ElementPage{}, fmt.Errorf("visibility: element range: list: %w", err)
	}
	els, err := x.complete(ctx, c, entries)
	if err != nil {
		return ElementPage{}, err
	}
	page := ElementPage{Elements: els, More: len(entries) == limit}
	if len(entries) > 0 {
		page.Last = entries[len(entries)-1].Address
	}
	return page, nil
}

// compiled is one call's scope: the store scope, its plan, the roll-up
// names per entity type, and which types fold their stored elements.
type compiled struct {
	scope   store.ElementScope
	plan    store.ElementRangePlan
	rollups map[string][]string
	// stored holds, per entity type of an entity range or relation type of a
	// relation range, whether its plan is [store.FoldStored].
	stored map[string]bool
}

// foldsStored reports whether every type whose rows are elements of the
// range folds its stored rows.
func (c compiled) foldsStored() bool {
	for _, s := range c.stored {
		if !s {
			return false
		}
	}
	return true
}

// storedEntry reports whether en's type folds its stored rows, so its stored
// hash is the reader's view hash.
func (c compiled) storedEntry(en store.RangeEntry) bool {
	if c.plan.Range.Relations {
		return c.stored[en.Relation.Type]
	}
	return c.stored[en.Type]
}

// compile builds the store scope of types for the request on ctx and plans r
// under it. It checks the bound request first, so a ctx without one fails
// whatever types are named.
func (x *Elements) compile(ctx context.Context, types ElementTypes, r store.ElementRange) (compiled, error) {
	if _, err := x.reads.Bind(ctx); err != nil {
		return compiled{}, fmt.Errorf("visibility: element range: %w", err)
	}
	c := compiled{
		scope: store.ElementScope{
			Entities:  map[string]store.ElementTypeScope{},
			Relations: map[string]store.RelationElementScope{},
		},
		rollups: map[string][]string{},
		stored:  map[string]bool{},
	}
	for _, typ := range compactSorted(slices.Clone(types.Entities)) {
		names := x.rollups.Names(typ)
		ts, admitted, err := x.typeScope(ctx, typ, names, r.Relations)
		if err != nil {
			return compiled{}, err
		}
		if admitted {
			c.scope.Entities[typ] = ts
			c.rollups[typ] = names
		}
	}
	for _, typ := range compactSorted(slices.Clone(types.Relations)) {
		if x.policy.RelationMetaGated(typ) {
			return compiled{}, fmt.Errorf("%w: %q", ErrRelationMetaGated, typ)
		}
		c.scope.Relations[typ] = store.RelationElementScope{Visibility: &store.FieldVisibility{}}
	}
	plan, err := store.PlanElementRange(c.scope, r)
	if err != nil {
		return compiled{}, err
	}
	c.plan = plan
	if r.Relations {
		for _, rp := range plan.Relations {
			c.stored[rp.Type] = rp.Fold == store.FoldStored
		}
	} else {
		for _, tp := range plan.Types {
			c.stored[tp.Type] = tp.Fold == store.FoldStored
		}
	}
	return c, nil
}

// typeScope compiles one entity type: the row gate from the reader's read
// query, as [Searcher] does, and whether its rows fold their stored
// elements. admitted is false when the reader reads no row. In a relation
// range (endpoint) the type only gates endpoints, so its fields are not
// compiled.
func (x *Elements) typeScope(
	ctx context.Context, typ string, rollups []string, endpoint bool,
) (store.ElementTypeScope, bool, error) {
	rqr, err := x.reads.ReadQueryFor(ctx, typ)
	if err != nil {
		return store.ElementTypeScope{}, false, err
	}
	var ts store.ElementTypeScope
	switch {
	case rqr.DenyAll:
		return store.ElementTypeScope{}, false, nil
	case rqr.Faces != nil && len(rqr.Faces) == 0:
		return store.ElementTypeScope{}, false, nil
	case rqr.AllowAll:
		ts = store.ElementTypeScope{AllowAll: true, Faces: rqr.Faces}
	case rqr.Query != nil:
		ts = store.ElementTypeScope{Query: rqr.Query, Faces: rqr.Faces}
	default:
		return store.ElementTypeScope{}, false, fmt.Errorf("visibility: element range: read query for %q has no branch", typ)
	}
	if endpoint || len(rollups) > 0 {
		// An endpoint type folds nothing. A type with roll-up names gets a
		// nil Visibility, like any type that hides a field (hidesSome).
		return ts, true, nil
	}
	hides, err := x.hidesSome(ctx, typ)
	if err != nil {
		return store.ElementTypeScope{}, false, err
	}
	if !hides {
		ts.Visibility = &store.FieldVisibility{}
	}
	return ts, true, nil
}

// hidesSome reports whether the field scope can hide a property of typ from
// the reader on some row. A type that hides none folds its stored elements.
// Any other type gets a nil Visibility, so the store folds none of its rows
// and Elements recomputes all of them. The per-row conditions are never sent
// to the store: its per-row verdict would only say which rows hold a hidden
// value, and nothing here may depend on that.
func (x *Elements) hidesSome(ctx context.Context, typ string) (bool, error) {
	fs, err := x.policy.SearchFieldScope(ctx, typ)
	if err != nil {
		return false, fmt.Errorf("visibility: element range: field scope for %q: %w", typ, err)
	}
	if fs == nil {
		return false, nil
	}
	for _, prop := range fs.Gated {
		c, _ := visibleWhen(fs, prop)
		if !slices.Contains(c.Any, nil) { // no grant shows it on every row
			return true, nil
		}
	}
	return false, nil
}

// complete returns the elements of entries in order. A row of a type folded
// from stored elements keeps its stored hash. Every other row, and a stored
// row whose element is not filled or does not parse, is loaded and hashed
// from its view. A row that is gone, or fails the gate when it is loaded, is
// left out.
func (x *Elements) complete(ctx context.Context, c compiled, entries []store.RangeEntry) ([]reconcile.Element, error) {
	kind := reconcile.KindEntity
	if c.plan.Range.Relations {
		kind = reconcile.KindRelation
	}
	hashes := make(map[string]reconcile.Element, len(entries))
	var load []store.RangeEntry
	for _, en := range entries {
		if en.Pending || !c.storedEntry(en) {
			load = append(load, en)
			continue
		}
		h, err := canonical.ParseDigest(en.Hash)
		if err != nil {
			slog.WarnContext(ctx, "visibility: element range: stored element hash does not parse; recomputing",
				"address", en.Address, "error", err)
			load = append(load, en)
			continue
		}
		hashes[en.Address] = reconcile.Element{Kind: kind, Address: en.Address, Hash: h}
	}
	var views map[string]reconcile.Element
	var err error
	if c.plan.Range.Relations {
		views, err = x.relationViews(ctx, c, load)
	} else {
		views, err = x.entityViews(ctx, c, load)
	}
	if err != nil {
		return nil, err
	}
	out := make([]reconcile.Element, 0, len(entries))
	for _, en := range entries {
		if el, ok := hashes[en.Address]; ok {
			out = append(out, el)
		} else if el, ok := views[en.Address]; ok {
			out = append(out, el)
		} // else gone or no longer readable
	}
	return out, nil
}

// entityViews loads, re-gates, redacts and hashes the entity rows of load,
// keyed by address.
func (x *Elements) entityViews(
	ctx context.Context, c compiled, load []store.RangeEntry,
) (map[string]reconcile.Element, error) {
	want := map[string]map[entity.Ref]bool{}
	for _, en := range load {
		if want[en.Type] == nil {
			want[en.Type] = map[entity.Ref]bool{}
		}
		want[en.Type][en.Ref] = true
	}
	var rows []*entity.Entity
	for _, tp := range c.plan.Types {
		refs := want[tp.Type]
		if len(refs) == 0 {
			continue
		}
		got, err := x.loadEntities(ctx, tp, refs)
		if err != nil {
			return nil, err
		}
		rows = append(rows, got...)
	}
	ctx = primePage(ctx, x.policy, rows)
	out := make(map[string]reconcile.Element, len(rows))
	for _, e := range rows {
		view := *Redact(ctx, x.policy, e)
		view.Properties = withoutNames(view.Properties, c.rollups[e.Type])
		el := reconcile.EntityElement(view)
		out[el.Address] = el
	}
	return out, nil
}

// loadEntities reads the rows refs of tp's type in one query and keeps those
// tp's gate and face allowlist still admit.
func (x *Elements) loadEntities(
	ctx context.Context, tp store.TypePlan, refs map[entity.Ref]bool,
) ([]*entity.Entity, error) {
	ids := make([]string, 0, len(refs))
	for ref := range refs {
		ids = append(ids, ref.ID)
	}
	slices.Sort(ids)
	ids = slices.Compact(ids)
	var rows []*entity.Entity
	for e, err := range x.rows.ListEntities(ctx, store.EntityQuery{Type: tp.Type, IDs: ids, Faces: store.AllFaces()}) {
		if err != nil {
			return nil, fmt.Errorf("visibility: element range: load %q rows: %w", tp.Type, err)
		}
		if e.Type == tp.Type && refs[e.Ref()] && tp.Admits(e.Face) {
			rows = append(rows, e)
		}
	}
	if tp.Gate == nil || len(rows) == 0 {
		return rows, nil
	}
	matched, err := x.rows.MatchingFaces(ctx, *tp.Gate, ids)
	if err != nil {
		return nil, fmt.Errorf("visibility: element range: gate %q rows: %w", tp.Type, err)
	}
	return slices.DeleteFunc(rows, func(e *entity.Entity) bool {
		return !slices.Contains(matched[e.ID], e.Face)
	}), nil
}

// relationViews loads and hashes the relation rows of load (rows whose
// stored element is not filled or does not parse), keyed by address. A
// relation type is in the scope only when no meta of it is ever hidden, so
// its view is the stored row.
func (x *Elements) relationViews(
	ctx context.Context, c compiled, load []store.RangeEntry,
) (map[string]reconcile.Element, error) {
	want := map[string]map[entity.RelationKey]bool{}
	for _, en := range load {
		if !c.storedEntry(en) {
			// Only a hand-built scope has such a type; Elements never builds one.
			return nil, fmt.Errorf("%w: %q", ErrRelationMetaGated, en.Relation.Type)
		}
		if want[en.Relation.Type] == nil {
			want[en.Relation.Type] = map[entity.RelationKey]bool{}
		}
		want[en.Relation.Type][en.Relation] = true
	}
	out := map[string]reconcile.Element{}
	for _, typ := range slices.Sorted(maps.Keys(want)) {
		keys := want[typ]
		var froms, tos []string
		for k := range keys {
			froms = append(froms, k.From)
			tos = append(tos, k.To)
		}
		q := store.RelationQuery{
			Type: typ, Direction: store.DirectionOutgoing,
			EntityIDs: compactSorted(froms), PeerIDs: compactSorted(tos),
		}
		found := 0
		for rel, err := range x.rows.ListRelations(ctx, q) {
			if err != nil {
				return nil, fmt.Errorf("visibility: element range: load %q relations: %w", typ, err)
			}
			if keys[rel.Identity()] {
				el := reconcile.RelationElement(*rel)
				out[el.Address] = el
				found++
				if found == len(keys) {
					break // the from-to product can hold many more edges
				}
			}
		}
	}
	return out, nil
}

// withoutNames returns props without names, copying only when one is set.
// props may be a store row's map and is never changed.
func withoutNames(props map[string]any, names []string) map[string]any {
	if !slices.ContainsFunc(names, func(n string) bool { _, ok := props[n]; return ok }) {
		return props
	}
	out := maps.Clone(props)
	for _, n := range names {
		delete(out, n)
	}
	return out
}

func compactSorted(s []string) []string {
	slices.Sort(s)
	return slices.Compact(s)
}

// SumHeld folds elements a caller already holds into the count and
// fingerprint that [Elements.Fingerprint] returns for the same rows, the
// fingerprint in its 32-hex-digit wire form. rows yields each element's
// address and view hash; relations says whether they are relations or
// entities. It is for a local side of a diff that is not the store, such as
// the remote pull's ledger, which must fingerprint exactly as this package
// does.
func SumHeld(relations bool, rows iter.Seq2[string, canonical.Digest]) (count uint64, fingerprint string) {
	kind := reconcile.KindEntity
	if relations {
		kind = reconcile.KindRelation
	}
	var rg reconcile.Range
	for addr, hash := range rows {
		rg = rg.Add(reconcile.Element{Kind: kind, Address: addr, Hash: hash})
	}
	return rg.Count, rg.Sum.String()
}
