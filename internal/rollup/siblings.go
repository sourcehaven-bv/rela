package rollup

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// Siblings is the compiled form of an earlier-siblings roll-up (design gap
// 2): an entity is behind when, under a parent that meets WhenParent, a
// sibling that meets Where comes before it in the incoming order of
// Relation. With Climb, each ancestor along Relation within Climb hops is
// judged too.
type Siblings struct {
	// Relation is the canonical, identity-scoped relation whose incoming
	// side is ordered by [metamodel.OrderPropertyIn].
	Relation string
	// Parents are the types Relation leads to; Members the types it leads
	// from, so the types a sibling can have. Both are sorted.
	Parents []string
	Members []string
	// Where are the equalities a sibling must meet, per member type, and
	// WhenParent those a parent must meet, per parent type. A type without
	// an entry has no condition.
	Where      map[string][]store.PropPredicate
	WhenParent map[string][]store.PropPredicate
	// Climb is the most hops an ancestor may be above the entity; zero
	// judges the entity alone.
	Climb int
	// CountSiblings is set for count, which counts the earlier siblings
	// rather than whether one exists.
	CountSiblings bool
}

// compileSiblingsColumn checks an earlier-siblings roll-up (design section
// 4.2) and compiles it, returning every problem found.
func compileSiblingsColumn(
	meta *metamodel.Metamodel, typeName, name string, fn metamodel.RollupFunc, r *metamodel.RollupDef, at string,
) (col Column, problems []string) {
	relName := strings.TrimSpace(r.SiblingsBefore)
	at = fmt.Sprintf("%s siblings_before %s", at, relName)
	switch fn {
	case metamodel.RollupExists:
	case metamodel.RollupCount:
		if r.Climb {
			problems = append(problems, at+": count cannot climb; use exists")
		}
	case metamodel.RollupMin, metamodel.RollupMax, metamodel.RollupSum, metamodel.RollupLatest,
		metamodel.RollupAvg, metamodel.RollupLatestRow, metamodel.RollupBodyTasks:
		problems = append(problems, fmt.Sprintf("%s: siblings_before folds exists or count, not %s", at, fn))
	}
	if r.Climb && (r.MaxDepth < 1 || r.MaxDepth > MaxAlongDepth) {
		problems = append(problems, fmt.Sprintf("%s: climb needs max_depth, from 1 to %d", at, MaxAlongDepth))
	}
	def, problem := siblingsRelation(meta, typeName, relName)
	if problem != "" {
		return Column{}, append(problems, at+": "+problem)
	}
	s := &Siblings{
		Relation: relName,
		Parents:  slices.Sorted(slices.Values(def.To)),
		Members:  slices.Sorted(slices.Values(def.From)),
		Where:    map[string][]store.PropPredicate{}, WhenParent: map[string][]store.PropPredicate{},
	}
	if r.Climb {
		s.Climb = r.MaxDepth
	}
	s.CountSiblings = fn == metamodel.RollupCount
	problems = append(problems, siblingsConditions(meta, s.Members, r.Where, s.Where, at+": where")...)
	problems = append(problems, siblingsConditions(meta, s.Parents, r.WhenParent, s.WhenParent, at+": when_parent")...)
	return Column{Property: name, Func: fn, Siblings: s}, problems
}

// siblingsRelation is the relation a siblings_before names, or why it
// cannot order typeName's siblings.
func siblingsRelation(
	meta *metamodel.Metamodel, typeName, relName string,
) (def *metamodel.RelationDef, problem string) {
	def, ok := meta.GetRelationDef(relName)
	switch {
	case !ok:
		if owner, inverse := meta.InverseOwner(relName); inverse {
			return nil, fmt.Sprintf("%q is the inverse of %q; name the relation itself", relName, owner)
		}
		return nil, fmt.Sprintf("unknown relation %q", relName)
	case def.IncomingOrderProperty() == "":
		return nil, fmt.Sprintf("relation %q does not order its incoming side (orderable: incoming or both)", relName)
	case def.MaxOutgoing == nil || *def.MaxOutgoing != 1:
		return nil, fmt.Sprintf("relation %q must declare max_outgoing: 1", relName)
	case metamodel.IsContentScoped(meta, relName):
		return nil, fmt.Sprintf("relation %q is content-scoped; siblings follow identity-scoped relations only", relName)
	case len(def.From) == 0 || len(def.To) == 0:
		return nil, fmt.Sprintf("relation %q must declare its from and to types", relName)
	case !slices.Contains(def.From, typeName):
		return nil, fmt.Sprintf("relation %q does not lead from %q", relName, typeName)
	}
	for _, t := range append(slices.Clone(def.From), def.To...) {
		if _, ok := meta.Entities[t]; !ok {
			return nil, fmt.Sprintf("relation %q names %q, which is not an entity type", relName, t)
		}
	}
	return def, ""
}

// siblingsConditions compiles the equalities of decl into out for each of
// types, with the where rules: every key a stored scalar property of every
// type, every value one it can hold. It returns the problems found.
func siblingsConditions(
	meta *metamodel.Metamodel, types []string, decl map[string]string, out map[string][]store.PropPredicate, at string,
) []string {
	var problems []string
	for _, typ := range types {
		props := meta.Entities[typ].Properties
		for _, raw := range slices.Sorted(maps.Keys(decl)) {
			key := strings.TrimSpace(raw)
			rp, ok := props[key]
			switch {
			case !ok:
				problems = append(problems, fmt.Sprintf("%s %q is not a property of %q", at, key, typ))
				continue
			case rp.Rollup != nil || rp.Computed != "" || rp.List:
				problems = append(problems, fmt.Sprintf("%s %q on %q is not a stored scalar", at, key, typ))
				continue
			}
			pred, problem := wherePredicate(meta, key, strings.TrimSpace(decl[raw]), rp)
			if problem != "" {
				problems = append(problems, fmt.Sprintf("%s %q on %q: %s", at, key, typ, problem))
				continue
			}
			out[typ] = append(out[typ], pred)
		}
	}
	return problems
}
