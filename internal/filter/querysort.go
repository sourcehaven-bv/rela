package filter

import (
	"fmt"
	"sort"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// QuerySort orders rows the way the STORE orders them, so a page served from
// the database and the same page sorted in Go read identically (TKT-9OFGH4).
//
// The rule is [store.GraphQuery.OrderBy]'s documented contract, which
// graphquerynaive.Order already implements and pgstore already emits:
//
//   - compare the STRING form of the value, byte-wise;
//   - except an enum (or any property declaring `values:`), which compares by
//     declared position so a workflow enum reads in workflow order;
//   - a row without the property sorts as the largest value — last ascending,
//     first descending, SQL's default null placement;
//   - id ascending as the final tiebreak, in BOTH directions.
//
// # Why byte-wise rather than type-aware
//
// Properties are stored as JSON and every backend orders them as text: pgstore
// emits `(properties ->> $n) COLLATE "C"`, and sqlite/fsstore/memstore route
// GraphQuery through graphquerynaive. Ordering that a backend cannot express
// cannot be pushed down, and a sort that cannot be pushed down forces the whole
// type to be loaded and sorted in Go — which defeats paging and is the cost
// this rule exists to avoid.
//
// So the comparator conforms to SQL rather than the reverse. The visible
// consequences are deliberate: strings compare case-sensitively and without
// numeric awareness ("Zebra" before "apple", "item10" before "item9"), and a
// date declaring a non-default `format:` sorts lexically rather than
// chronologically. Store dates ISO-formatted if you need chronological order.
//
// The enum rank is the one semantic that IS expressible — as an indexable
// `CASE` — which is why it survives while the other type-aware comparisons do
// not. [Sort] and [SortMulti]'s per-type comparisons remain for callers that
// order a result set they already hold; they are not on the query path.
type QuerySort struct {
	// ranks maps a property name to its declared value order. Absent means
	// "compare byte-wise"; present means "compare by position, unknown values
	// last". Built once per sort rather than per comparison.
	ranks map[string]map[string]int
}

// NewQuerySort resolves the declared value order of every property named by
// specs, across every entity type in defs.
//
// Nil: a nil meta or defs is accepted — it yields byte-wise comparison for
// every property, which is the correct answer when no schema is available.
//
// A property declaring different values on two entity types in the same result
// set gets NO rank, and falls back to byte-wise. Two conflicting ranks cannot
// both be right, and a silent choice between them would order a mixed list by
// whichever type happened to be seen first.
func NewQuerySort(
	specs []SortSpec,
	defs map[string]*metamodel.EntityDef,
	meta *metamodel.Metamodel,
) *QuerySort {
	qs := &QuerySort{ranks: map[string]map[string]int{}}
	if len(defs) == 0 {
		return qs
	}
	conflicting := map[string]bool{}
	for _, spec := range specs {
		if spec.Property == "" || isVirtualSortProperty(spec.Property) {
			continue
		}
		for _, def := range defs {
			if def == nil {
				continue
			}
			pd, ok := def.Properties[spec.Property]
			if !ok {
				continue
			}
			index := buildEnumIndex(&pd, meta)
			if index == nil {
				continue
			}
			switch existing, seen := qs.ranks[spec.Property]; {
			case !seen:
				qs.ranks[spec.Property] = index
			case !sameRank(existing, index):
				conflicting[spec.Property] = true
			}
		}
	}
	for property := range conflicting {
		delete(qs.ranks, property)
	}
	return qs
}

// Ranks returns the declared value order resolved for property, or nil when it
// compares byte-wise. The map must not be modified.
func (q *QuerySort) Ranks(property string) map[string]int {
	if q == nil {
		return nil
	}
	return q.ranks[property]
}

// QuerySortApply orders items in place by specs, most significant key first.
//
// A free function rather than a method because Go does not allow type
// parameters on methods, and the whole point is to sort the caller's own slice
// without converting it.
//
// One pass with a composite comparator, NOT one stable pass per key: the
// tiebreak is part of the ordering rather than an accident of stability, which
// is what makes a paged read reproducible. Ties resolved by id mean a row
// cannot appear on two pages or on none.
func QuerySortApply[T any](q *QuerySort, items []T, access Accessor[T], specs []SortSpec) {
	if q == nil || len(specs) == 0 || len(items) < 2 {
		return
	}
	sort.SliceStable(items, func(i, j int) bool {
		ri, rj := access(items[i]), access(items[j])
		for _, spec := range specs {
			if c := q.compareSpec(ri, rj, spec); c != 0 {
				return c < 0
			}
		}
		return ri.ID < rj.ID
	})
}

// compareSpec returns -1, 0 or +1 for one sort key, with direction applied.
//
// Direction inverts the KEY comparison only. The id tiebreak in [QuerySortApply]
// stays ascending in both directions, matching `ORDER BY <key> DESC, id ASC`,
// which every backend emits. Inverting the whole comparison instead — the
// `return !less` shape this replaces — is not a valid ordering at all: for two
// equal keys it reports both "i before j" and "j before i", which makes
// sort.SliceStable produce input-dependent garbage and silently reverses a
// secondary key under a descending primary.
func (q *QuerySort) compareSpec(a, b Record, spec SortSpec) int {
	c := q.compareKey(a, b, spec.Property)
	if spec.IsDescending() {
		return -c
	}
	return c
}

func (q *QuerySort) compareKey(a, b Record, property string) int {
	if property == "id" {
		return strings.Compare(a.ID, b.ID)
	}
	sa, oka := sortValue(a, property)
	sb, okb := sortValue(b, property)
	if !oka || !okb {
		switch {
		case oka == okb:
			return 0
		case oka:
			return -1 // a present, b absent: the present value is smaller
		default:
			return 1
		}
	}
	if rank := q.ranks[property]; rank != nil {
		return compareRanked(sa, sb, rank)
	}
	return strings.Compare(sa, sb)
}

// compareRanked orders two values by declared position, with any value the
// schema does not declare sorting after every declared one and byte-wise among
// its peers. Matches compareEnums, and the `ELSE` arm of the SQL `CASE`.
func compareRanked(a, b string, rank map[string]int) int {
	ia, knownA := rank[a]
	ib, knownB := rank[b]
	switch {
	case knownA && knownB:
		return cmpInt(ia, ib)
	case knownA:
		return -1
	case knownB:
		return 1
	default:
		return strings.Compare(a, b)
	}
}

func cmpInt(a, b int) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	default:
		return 0
	}
}

// sortValue reads a sort key the way SQL's `->>` does: a key that is missing OR
// holds JSON null is "no value" (SQL NULL, the largest), never the text
// "<nil>". Everything else is rendered to the text form `->>` would yield, so a
// number, a bool and a list all sort as the string the database would compare.
func sortValue(r Record, property string) (string, bool) {
	v, ok := r.Properties[property]
	if !ok || v == nil {
		return "", false
	}
	if s, isString := v.(string); isString {
		return s, true
	}
	return fmt.Sprintf("%v", v), true
}

// isVirtualSortProperty reports whether name addresses something other than a
// stored property. Only "id" is honored on the query path — it is a real
// column on every backend. "modified" is NOT: it has no stored column, so no
// backend can order by it, and accepting it here would produce an ordering the
// pushed path could never reproduce.
func isVirtualSortProperty(name string) bool {
	return name == "id" || name == "modified"
}

func sameRank(a, b map[string]int) bool {
	if len(a) != len(b) {
		return false
	}
	for k, av := range a {
		if bv, ok := b[k]; !ok || av != bv {
			return false
		}
	}
	return true
}
