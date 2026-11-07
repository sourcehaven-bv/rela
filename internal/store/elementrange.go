package store

import (
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/Sourcehaven-BV/rela/internal/entity"
)

// The element range read (TKT-IQU106C) answers, for one reader, the count
// and fingerprint sum of the reconciliation elements in an address range,
// and lists them. It is what rela-to-rela sync compares ranges with
// (FEAT-CMVZQFI); the element model is the reconcile package's.
//
// Every backend has an element range reader with these two methods:
//
//	RangeFingerprint(ctx, ElementScope, ElementRange) (RangeSum, error)
//	ListRange(ctx, ElementScope, ElementRange, RangeList) ([]RangeEntry, error)
//
// The reader is a value of its own, built from the store at the wiring site:
// elementnaive.New for memstore and fsstore, pgstore.NewElementRanges and
// sqlitestore.NewElementRanges for the database backends, which answer each
// call in one statement over the stored element columns. It is not a store
// method, so no store decorator can hide it, and there is no producer-side
// interface: the consumer declares the two methods it needs.
// storetest.RunElementRangeTests pins all four backends.
//
// # The rules
//
// The scope is the reader's whole statement of what it may see, compiled
// from the ACL by the caller. The store enforces the reconcile package's
// rules against it, inside the query:
//
//   - A row the scope does not admit changes neither the count nor the sum.
//   - A relation is in range only when its head has a row the scope admits
//     and its tail row is admitted: the row at its FromFace, or any row of
//     the tail for an identity tail. This is visibility.EndpointsReadable's
//     rule, with the scope as the gate.
//   - A row's stored hash and limbs are folded or listed only when the
//     reader has no hidden field on that row ([ElementTypeScope.Visibility]).
//     Any other admitted row is pending: counted, never summed, listed
//     without a hash. On the database backends a row whose stored element is
//     still NULL is pending too. elementnaive stores nothing: it computes
//     each element from the row, so it has no NULL rows.
//
// Nothing is cached: every call reads the store. A result is one reader's
// and must not be reused for another.

// ElementScope says which rows one reader may see, per type. A type absent
// from a map has no rows in scope. The scope is server-derived from the ACL;
// it is never taken from a request.
type ElementScope struct {
	Entities  map[string]ElementTypeScope
	Relations map[string]RelationElementScope
}

// ElementTypeScope is the scope of one entity type. Its gate has the
// meaning of search.TypeScope's: AllowAll admits every row, Query admits the
// rows MatchingFaces returns for it with EntityType set to the scope key
// and Faces set to [AllFaces], and neither denies the type.
type ElementTypeScope struct {
	AllowAll bool
	// Query is the row gate. It is copied, never changed: ACL query
	// templates are shared across requests.
	Query *GraphQuery
	// Faces, when non-nil, admits only rows at these faces. It narrows the
	// gate and never widens Query.FaceIn. A non-nil empty list is refused,
	// because "no faces" must not read as "every face".
	Faces []entity.Face
	// Visibility says which fields the reader cannot see on the rows the
	// gate admits.
	//
	// Nil: accepted, and means the field visibility is not known, so every
	// row of the type is pending. That is the fail-closed default: a caller
	// that forgot to compile the grants gets no stored hashes.
	// &FieldVisibility{} means the reader sees every field.
	Visibility *FieldVisibility
}

// FieldVisibility lists the properties a reader may not see on some rows.
//
// It covers properties only. A body is never redacted today; if body
// redaction is added, this read is on the list in visibility/policyreader.go
// and needs a hidden-body condition here.
//
// The view a stored hash stands in for is the stored row without computed
// roll-ups. A caller lists every roll-up property name of the type as a
// HiddenField with an empty condition, so a row that stores a value under
// such a name is pending: no reader is served that stored value.
type FieldVisibility struct {
	Hidden []HiddenField
}

// HiddenField is a property the reader sees only on the rows where Visible
// holds. A row on which the property key is absent has nothing hidden. A
// present key, even holding JSON null, counts as present.
type HiddenField struct {
	Property string
	// Visible holds on the rows where the reader may see the property; the
	// zero value (no query) holds on no row. The queries are evaluated as
	// [RowCondition] says.
	Visible RowCondition
}

// RelationElementScope is the scope of one relation type. Whether a
// relation is admitted depends on its endpoints alone (see the rules above).
type RelationElementScope struct {
	// Visibility is accepted as nil, which makes every row of the type
	// pending. Non-nil must have no Hidden entries, and is allowed only when
	// no role of the policy has a meta `visible:` block for this relation
	// type on any source type: relation meta visibility is keyed by source
	// type, face and role, so one flag per relation type cannot say
	// more than "nothing is ever hidden".
	Visibility *FieldVisibility
}

// ElementRange is a range of element addresses of one kind: entities, or
// relations when Relations is set. Lo is inclusive and Hi exclusive, compared
// bytewise; Unbounded drops the upper bound. The zero value is the empty
// range.
type ElementRange struct {
	Relations bool
	Lo, Hi    string
	Unbounded bool
}

// RangeList bounds a ListRange call. Limit is 1 to [MaxRangeList].
// PendingOnly lists only the pending rows, which the caller completes with
// view hashes.
type RangeList struct {
	Limit       int
	PendingOnly bool
}

// MaxRangeList is the largest [RangeList.Limit].
const MaxRangeList = 10000

// MaxRangeBound is the longest [ElementRange] bound, in bytes. A bound is
// longer than any real address only when a peer sends junk.
const MaxRangeBound = 4096

// RangeSum is the count and fingerprint sum of a range.
//
// Count counts every admitted row, pending ones included. LimbSums sums the
// stored fingerprint limbs of the non-pending rows column by column; the
// caller folds them with reconcile.FingerprintFromLimbSums and adds the
// fingerprints of the pending rows' view hashes.
//
// Pending is server-internal. Whether a row is pending says whether a
// hidden property is set on it, so it must never reach the wire; the json
// tag keeps it out of an encoded value.
type RangeSum struct {
	Count    uint64
	Pending  uint64 `json:"-"`
	LimbSums [4]uint64
}

// RangeEntry is one element in a listing, in bytewise address order.
//
// Hash is the stored hex element hash, empty exactly when Pending. Like
// [RangeSum.Pending], Pending is server-internal. For an entity, Type and
// Ref name the row; for a relation, Relation does.
//
// RangeFingerprint and ListRange read separately: a write between two calls
// can make them disagree. On the database backends one call is one
// statement; elementnaive makes several store reads per call, so even one of
// its calls can see a write land. A caller that lists a range and needs its
// sum derives the sum from the listing.
type RangeEntry struct {
	Address  string
	Hash     string
	Pending  bool `json:"-"`
	Type     string
	Ref      entity.Ref
	Relation entity.RelationKey
}

// FoldMode says which rows of a type may fold their stored element.
type FoldMode int

const (
	// FoldPending folds no row: every row is pending.
	FoldPending FoldMode = iota
	// FoldStored folds every row with a stored element.
	FoldStored
	// FoldPerRow folds a row with a stored element when no hidden field is
	// present on it or each present one is visible there.
	FoldPerRow
)

// ElementRangePlan is a validated range request in the form every backend
// executes. Build it with [PlanElementRange].
type ElementRangePlan struct {
	Range ElementRange
	// Types are the entity types to read, sorted. For an entity range they
	// are the admitted types whose address window meets the range; for a
	// relation range, every admitted type, as endpoint gates.
	Types []TypePlan
	// Relations are the relation types in scope, sorted. Empty for an
	// entity range.
	Relations []RelationPlan
}

// Empty reports whether the plan can admit no row, so a backend answers
// without reading.
func (p ElementRangePlan) Empty() bool {
	if !p.Range.Unbounded && p.Range.Lo >= p.Range.Hi {
		return true
	}
	if p.Range.Relations {
		return len(p.Relations) == 0 || len(p.Types) == 0
	}
	return len(p.Types) == 0
}

// TypePlan is one admitted entity type.
type TypePlan struct {
	Type string
	// Lo and Hi bound the type's addresses within the range: Lo inclusive,
	// Hi exclusive, both always set. Every address of the type starts with
	// Type + "/", so the window is [Type+"/", Type+"0") clipped to the range.
	// Meaningful for an entity range only.
	Lo, Hi string
	// Gate is the row gate as a MatchingFaces probe, or nil when every row
	// is admitted. It is a copy; changing it does not change the scope.
	Gate *GraphQuery
	// Faces, when non-nil, admits only rows at these faces (never empty).
	Faces []entity.Face
	Fold  FoldMode
	// Hidden is set under FoldPerRow.
	Hidden []HiddenField
}

// RelationPlan is one relation type in scope.
type RelationPlan struct {
	Type string
	// Fold is FoldStored or FoldPending.
	Fold FoldMode
}

// FoldTypes returns the relation types whose rows fold their stored element.
func (p ElementRangePlan) FoldTypes() []string {
	out := []string{}
	for _, r := range p.Relations {
		if r.Fold == FoldStored {
			out = append(out, r.Type)
		}
	}
	return out
}

// RelationTypes returns every relation type in scope.
func (p ElementRangePlan) RelationTypes() []string {
	out := make([]string, len(p.Relations))
	for i, r := range p.Relations {
		out[i] = r.Type
	}
	return out
}

// ValidateRangeList refuses a limit outside 1 to [MaxRangeList].
func ValidateRangeList(l RangeList) error {
	if l.Limit < 1 || l.Limit > MaxRangeList {
		return fmt.Errorf("%w: element range: limit %d is outside 1..%d", ErrInvalidQuery, l.Limit, MaxRangeList)
	}
	return nil
}

// PlanElementRange validates scope and r and returns the plan every backend
// executes, so the backends differ only in how they render it.
func PlanElementRange(scope ElementScope, r ElementRange) (ElementRangePlan, error) {
	if err := validateElementRange(r); err != nil {
		return ElementRangePlan{}, err
	}
	plan := ElementRangePlan{Range: r}
	for _, typ := range slices.Sorted(maps.Keys(scope.Entities)) {
		tp, admitted, err := planType(typ, scope.Entities[typ])
		if err != nil {
			return ElementRangePlan{}, err
		}
		if !admitted {
			continue
		}
		if !r.Relations {
			var ok bool
			if tp.Lo, tp.Hi, ok = typeWindow(typ, r); !ok {
				continue
			}
		}
		plan.Types = append(plan.Types, tp)
	}
	for _, typ := range slices.Sorted(maps.Keys(scope.Relations)) {
		rs := scope.Relations[typ]
		if err := validateScopeKey("relation", typ); err != nil {
			return ElementRangePlan{}, err
		}
		fold := FoldPending
		if rs.Visibility != nil {
			if len(rs.Visibility.Hidden) > 0 {
				return ElementRangePlan{}, fmt.Errorf(
					"%w: element range: relation type %q: hidden relation fields are not supported",
					ErrInvalidQuery, typ)
			}
			fold = FoldStored
		}
		if r.Relations {
			plan.Relations = append(plan.Relations, RelationPlan{Type: typ, Fold: fold})
		}
	}
	return plan, nil
}

func validateElementRange(r ElementRange) error {
	if len(r.Lo) > MaxRangeBound || len(r.Hi) > MaxRangeBound {
		return fmt.Errorf("%w: element range: a bound is longer than %d bytes", ErrInvalidQuery, MaxRangeBound)
	}
	// An address is valid UTF-8 without NUL. PostgreSQL refuses anything
	// else in a text parameter, so refusing it here keeps every backend's
	// answer to the same peer input the same.
	for _, b := range []string{r.Lo, r.Hi} {
		if !utf8.ValidString(b) || strings.IndexByte(b, 0) >= 0 {
			return fmt.Errorf("%w: element range: a bound is not valid UTF-8 or holds NUL", ErrInvalidQuery)
		}
	}
	if !r.Unbounded && r.Lo > r.Hi {
		return fmt.Errorf("%w: element range: lower bound sorts after upper bound", ErrInvalidQuery)
	}
	return nil
}

func validateScopeKey(kind, typ string) error {
	if typ == "" || typ == "*" {
		return fmt.Errorf("%w: element range: %s scope key %q must name one type", ErrInvalidQuery, kind, typ)
	}
	return nil
}

// planType validates one entity type's scope. admitted is false for a
// denied type.
func planType(typ string, ts ElementTypeScope) (tp TypePlan, admitted bool, err error) {
	if err := validateScopeKey("entity", typ); err != nil {
		return TypePlan{}, false, err
	}
	fail := func(msg string) error {
		return fmt.Errorf("%w: element range: entity type %q: %s", ErrInvalidQuery, typ, msg)
	}
	otherType := func(q *GraphQuery) bool { return q != nil && q.EntityType != "" && q.EntityType != typ }
	switch {
	case ts.AllowAll && ts.Query != nil:
		return TypePlan{}, false, fail("AllowAll and Query are exclusive")
	case otherType(ts.Query):
		return TypePlan{}, false, fail("the gate query names another type")
	case ts.Faces != nil && len(ts.Faces) == 0:
		return TypePlan{}, false, fail("an empty face list admits nothing; deny the type instead")
	}
	if ts.Visibility != nil {
		for _, h := range ts.Visibility.Hidden {
			if h.Property == "" {
				return TypePlan{}, false, fail("a hidden field names no property")
			}
			if slices.ContainsFunc(h.Visible.Any, otherType) {
				return TypePlan{}, false, fail("a hidden field condition names another type")
			}
		}
	}
	if !ts.AllowAll && ts.Query == nil {
		return TypePlan{}, false, nil
	}
	tp = TypePlan{Type: typ, Faces: slices.Clone(ts.Faces)}
	if ts.Query != nil {
		probe := rowProbe(typ, ts.Query)
		tp.Gate = &probe
	}
	switch {
	case ts.Visibility == nil:
		tp.Fold = FoldPending
	case len(ts.Visibility.Hidden) == 0:
		tp.Fold = FoldStored
	default:
		tp.Fold = FoldPerRow
		tp.Hidden = slices.Clone(ts.Visibility.Hidden)
	}
	return tp, true, nil
}

// typeWindow clips the range to typ's addresses, or reports !ok when they
// do not meet.
func typeWindow(typ string, r ElementRange) (lo, hi string, ok bool) {
	lo, hi = typ+"/", typ+"0" // '0' is the byte after '/'
	if r.Lo > lo {
		lo = r.Lo
	}
	if !r.Unbounded && r.Hi < hi {
		hi = r.Hi
	}
	return lo, hi, lo < hi
}

// Admits reports whether the type's face allowlist admits face.
func (tp TypePlan) Admits(face entity.Face) bool {
	return tp.Faces == nil || slices.Contains(tp.Faces, face)
}

// Probes returns the type's gate and every hidden-field condition as the
// MatchingFaces probes they are evaluated as, so a backend can check their
// shape before it renders them. A backend refuses the request when any probe
// fails the shape check, as the reference does.
func (tp TypePlan) Probes() []GraphQuery {
	var out []GraphQuery
	if tp.Gate != nil {
		out = append(out, *tp.Gate)
	}
	for _, h := range tp.Hidden {
		for _, q := range h.Visible.Any {
			if q != nil {
				out = append(out, rowProbe(tp.Type, q))
			}
		}
	}
	return out
}

// rowProbe is q as the MatchingFaces probe for rows of typ, as
// [HoldingRows] builds it. It copies q; the template is shared.
func rowProbe(typ string, q *GraphQuery) GraphQuery {
	probe := *q
	probe.EntityType = typ
	probe.Faces = AllFaces()
	probe.FaceIn = slices.Clone(probe.FaceIn)
	probe.OrderBy, probe.Limit, probe.Offset = nil, 0, 0
	return probe
}

// RangeSumFromAggregates checks a backend's scanned range aggregates and
// converts them. A negative value or more pending rows than rows would mean
// a corrupt column; it fails rather than wrapping.
func RangeSumFromAggregates(count, pending int64, limbs [4]int64) (RangeSum, error) {
	if count < 0 || pending < 0 || pending > count {
		return RangeSum{}, fmt.Errorf("element range: bad counts %d/%d", count, pending)
	}
	sum := RangeSum{Count: uint64(count), Pending: uint64(pending)}
	for i, l := range limbs {
		if l < 0 {
			return RangeSum{}, errors.New("element range: negative limb sum")
		}
		sum.LimbSums[i] = uint64(l)
	}
	return sum, nil
}

// SetRowKey fills the row key of e from a backend's four key columns: type,
// id and face for an entity; from, from face, type and to for a relation.
func (e *RangeEntry) SetRowKey(relations bool, k1, k2, k3, k4 string) {
	if relations {
		e.Relation = entity.RelationKey{From: k1, FromFace: entity.Face(k2), Type: k3, To: k4}
		return
	}
	e.Type = k1
	e.Ref = entity.Ref{ID: k2, Face: entity.Face(k3)}
}
