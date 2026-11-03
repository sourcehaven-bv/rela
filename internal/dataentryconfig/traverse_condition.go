package dataentryconfig

import (
	"slices"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/filter"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// TraverseTargetTypes returns the entity types one traverse step can collect:
// the `to:` types of a followed relation, the `from:` types of a relation
// followed incoming, and every type for a `mentioned_by` step. Nil when the
// relation is unknown, which validateViews reports.
//
// The result may share the metamodel's backing array; do not modify it.
func TraverseTargetTypes(t ViewTraverse, meta *metamodel.Metamodel) []string {
	if t.MentionedBy {
		return allEntityTypes(meta)
	}
	relType := t.Follow
	if relType == "" {
		relType = t.FollowIncoming
	}
	relDef, ok := meta.GetRelationDef(relType)
	if !ok {
		return nil
	}
	if t.Follow != "" {
		return relDef.To
	}
	return relDef.From
}

// TraverseConditionTypes returns the entity types a step's `condition:` is
// compiled for: [TraverseTargetTypes], narrowed by a `where:` on `type`.
// `where:` runs first, so a type it excludes never reaches the condition, and
// a condition need not compile for it.
//
// The result is a new slice.
func TraverseConditionTypes(t ViewTraverse, meta *metamodel.Metamodel) []string {
	types := TraverseTargetTypes(t, meta)
	f, err := filter.Parse(t.Where)
	if t.Where == "" || err != nil || f.Property != "type" {
		return slices.Clone(types)
	}
	var out []string
	for _, typ := range types {
		if filter.MatchValue(typ, f) {
			out = append(out, typ)
		}
	}
	return out
}

// TraverseConditionID names the compiled `condition:` of a traverse step for
// one entity type. A step compiles once per type it can collect, and the
// program depends on that type set too (see
// predicatefns.Evaluator.CompileForCollection), so all three are in the id.
//
// The id holds no view name. The view engine runs synthetic and side-panel
// view configs that have none, and two steps with the same condition over
// the same types share one program.
//
// A NUL cannot occur in an entity type name, so [SplitTraverseConditionID]
// recovers the type exactly.
func TraverseConditionID(entityType string, collectionTypes []string, condition string) string {
	types := slices.Clone(collectionTypes)
	slices.Sort(types)
	types = slices.Compact(types)
	return entityType + "\x00" + strings.Join(types, ",") + "\x00" + condition
}

// SplitTraverseConditionID returns the entity type of a [TraverseConditionID].
func SplitTraverseConditionID(id string) (entityType string, ok bool) {
	entityType, _, ok = strings.Cut(id, "\x00")
	return entityType, ok
}
