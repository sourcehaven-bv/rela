package predicatefns

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/predicate"
)

// ValidateTraversals checks every traversal a compiled program contains
// against the metamodel, resolving each hop's target entity type.
//
// The engine itself cannot do this: internal/predicate depends on nothing, so
// it accepts `related(entity, 'caused-by', {...})` without knowing whether
// `caused-by` exists or what it points at. This is where that is decided, and
// it is deliberately a LOAD-time check — a condition that cannot be resolved
// is a config error naming the relation, not a query that quietly matches
// nothing.
//
// # Why the type ascription is mandatory on a union
//
// A relation's target is a SET (`RelationDef.To`). When it names more than one
// type, a property reference has no single declared type to be compared
// against: in the live tickets schema, `status` is declared on nearly every
// type with a DIFFERENT enum type each (`ticket_status`, `bug_status`,
// `concept_status`), so `'done'` may be valid for one target and meaningless
// for another. Rather than silently pick one, require `type =`.
//
// A single-target relation needs no ascription — there is nothing to
// disambiguate — which keeps the common case clean.
func ValidateTraversals(meta *metamodel.Metamodel, fromType string, prog *predicate.Program) error {
	if meta == nil || prog == nil {
		return nil
	}
	for _, spec := range prog.Traversals() {
		if err := validateTraversal(meta, fromType, spec); err != nil {
			return err
		}
	}
	return nil
}

func validateTraversal(meta *metamodel.Metamodel, fromType string, spec predicate.TraversalSpec) error {
	target, err := ResolveTraversalTarget(meta, fromType, spec)
	if err != nil {
		return err
	}
	return validateTraversalProps(meta, target, spec)
}

// ResolveTraversalTarget walks a traversal's hops and returns the entity type
// the FINAL hop lands on, or an error naming why it cannot be resolved.
//
// This is the ONE definition of how a chain resolves, shared by validation
// (which surfaces the error at config load) and by index derivation in
// internal/queryplan (which treats any error as "derive no index"). They must
// agree: if validation accepts a chain that derivation cannot resolve, the
// condition loads fine and its index is silently never created — the query
// then scans every row of the target type, which is the regression the
// derived index exists to prevent. Two copies of this walk WILL drift, and
// that is the direction they drift in.
func ResolveTraversalTarget(
	meta *metamodel.Metamodel, fromType string, spec predicate.TraversalSpec,
) (string, error) {
	if meta == nil {
		return "", errors.New("related: no metamodel")
	}
	current := fromType
	for i, relType := range spec.Path {
		def, ok := meta.GetRelationDef(relType)
		if !ok {
			return "", fmt.Errorf("related: unknown relation type %q", relType)
		}
		// The FROM side must admit the type we are standing on. Checking it
		// turns a traversal that could never match into a load error naming
		// the relation, rather than a silently empty result.
		if current != "" && len(def.From) > 0 && !slices.Contains(def.From, current) {
			return "", fmt.Errorf("related: relation %q does not start from %q (declared from: %s)",
				relType, current, strings.Join(def.From, ", "))
		}

		targets := def.To
		last := i == len(spec.Path)-1
		switch {
		case len(targets) == 0:
			return "", fmt.Errorf("related: relation %q declares no target type", relType)
		case len(targets) == 1:
			current = targets[0]
			if last && spec.EntityType != "" && spec.EntityType != current {
				return "", fmt.Errorf("related: type %q does not match the target of %q (%s)",
					spec.EntityType, relType, current)
			}
		default:
			// A union. Only the LAST hop can carry the ascription, so an
			// intermediate union is unresolvable and must be refused rather
			// than guessed.
			if !last {
				return "", fmt.Errorf(
					"related: intermediate relation %q has %d target types (%s) and cannot be "+
						"resolved; a chained hop must traverse single-target relations",
					relType, len(targets), strings.Join(targets, ", "))
			}
			if spec.EntityType == "" {
				return "", fmt.Errorf(
					"related: relation %q has %d target types (%s); add type='<one of them>' to "+
						"say which one is meant", relType, len(targets), strings.Join(targets, ", "))
			}
			if !slices.Contains(targets, spec.EntityType) {
				return "", fmt.Errorf("related: type %q is not a target of %q (declared to: %s)",
					spec.EntityType, relType, strings.Join(targets, ", "))
			}
			current = spec.EntityType
		}
	}
	return current, nil
}

// validateTraversalProps checks each constrained property is declared on the
// resolved target type and is pushdown-eligible.
//
// Refusing an ineligible property is load-bearing, not fussiness. The whole
// point of the form is that it pushes into SQL; a property the store cannot
// compare would have to be evaluated per row in Go, which is the N+1 the
// collection-read rules exist to prevent — and it would be invisible to the
// storetest.Counting budgets, because it happens above the store's query path.
func validateTraversalProps(meta *metamodel.Metamodel, targetType string, spec predicate.TraversalSpec) error {
	if targetType == "" {
		return nil
	}
	def, ok := meta.GetEntityDef(targetType)
	if !ok {
		return fmt.Errorf("related: unknown entity type %q", targetType)
	}
	for _, name := range spec.PropNames() {
		pd, declared := def.Properties[name]
		if !declared {
			return fmt.Errorf("related: %q has no property %q", targetType, name)
		}
		if pd.List {
			// A list property compares as membership, which IS expressible —
			// but the two backends render a list differently, so the answer
			// would be backend-dependent. Refuse until that is pinned.
			return fmt.Errorf("related: property %q on %q is a list and cannot be filtered in a traversal",
				name, targetType)
		}
		if _, ok := spec.Props[name].(predicate.String); !ok {
			return fmt.Errorf("related: property %q on %q must be compared against a string literal",
				name, targetType)
		}
	}
	return nil
}
