package dataentryconfig

import (
	"fmt"
	"slices"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// checkCardFieldRelatedProperty validates a kanban card field that shows a
// property of the related entities: `relation` together with `property`
// (TKT-8Z2LT3).
//
// The board's type must sit on the side of the relation the direction starts
// from, and the property must be declared on EVERY type at the other end. That
// is stricter than calendar chips, which accept a property found on any source:
// a board has one entity type, so every target type is a type its cards can
// actually link to, and a type lacking the property would leave those cards
// blank with no diagnostic.
//
// Returns nil for any other field shape. An unknown relation is reported by
// the caller; an ambiguous missing direction by [CheckAmbiguousDirection].
func checkCardFieldRelatedProperty(
	at string, f KanbanCardField, entityType string, meta *metamodel.Metamodel,
) []string {
	if f.Relation == "" || f.Property == "" {
		return nil
	}
	var errs []string
	if f.Display != "" {
		errs = append(errs, at+": display cannot be combined with a related property")
	}
	rel, ok := meta.GetRelationDef(f.Relation)
	if !ok {
		return errs
	}
	dir := f.Direction
	if dir == "" {
		inferred, res := InferDirection(entityType, f.Relation, meta)
		switch res {
		case DirectionAmbiguous:
			return errs
		case DirectionNoSide:
			// No direction was written, so do not blame one.
			return append(errs, fmt.Sprintf(
				"%s: entity type %q is on neither side of relation %q (from: %s; to: %s)",
				at, entityType, f.Relation, strings.Join(rel.From, ", "), strings.Join(rel.To, ", ")))
		case DirectionResolved, DirectionUnknown:
		}
		dir = inferred
	}

	ownSide, otherSide := rel.From, rel.To
	sideName := "from"
	if dir == DirectionIncoming {
		ownSide, otherSide = rel.To, rel.From
		sideName = "to"
	}
	if !slices.Contains(ownSide, entityType) {
		return append(errs, fmt.Sprintf(
			"%s: entity type %q is not a %s type of relation %q, so it has no %s %q edges (valid %s types: %s)",
			at, entityType, sideName, f.Relation, dir, f.Relation, sideName, strings.Join(ownSide, ", ")))
	}

	var missing []string
	for _, t := range otherSide {
		def, ok := meta.GetEntityDef(t)
		if !ok {
			continue // an unknown type in a relation is a metamodel error
		}
		if _, ok := def.Properties[f.Property]; !ok {
			missing = append(missing, t)
		}
	}
	if len(missing) > 0 {
		errs = append(errs, fmt.Sprintf(
			"%s: property %q is not declared on entity type %s, which %s %q edges can lead to",
			at, f.Property, quoteJoin(missing), dir, f.Relation))
	}
	return errs
}

// quoteJoin renders names as `"a", "b"` for an error message.
func quoteJoin(names []string) string {
	quoted := make([]string, len(names))
	for i, n := range names {
		quoted[i] = fmt.Sprintf("%q", n)
	}
	return strings.Join(quoted, ", ")
}
