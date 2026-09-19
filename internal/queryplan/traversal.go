package queryplan

import (
	"log/slog"
	"slices"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/predicate"
	"github.com/Sourcehaven-BV/rela/internal/predicatefns"
	"github.com/Sourcehaven-BV/rela/internal/search/searchparser"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// TraversalIndexSpecs derives the derived-query index specs a condition's
// traversals need, on the TRAVERSED-TO type.
//
// # Why this is a second spec rather than more properties on the first
//
// A derived index is PARTIAL on its own type:
//
//	CREATE INDEX ... ON entities ((properties->>'status'))
//	  WHERE type = 'concept' AND jsonb_typeof(properties->'status') = 'string';
//
// and [store.DerivedObjectSpec] carries exactly one Type. A traversal filters
// rows of the type at the far END of the relation, so the index it needs sits
// on a different type from the query's own — no number of extra Properties on
// the query's spec can produce it.
//
// Without this, an endpoint filter is CORRECT but unindexed: measured on
// PG 18.6, the join fell back to scanning every row of the target type
// (5000 scanned to find 10). With it the plan is an index scan. See
// TKT-RELTRV and the EXPLAIN test in pgstore.
//
// Only the FINAL hop's properties are indexable: intermediate hops are
// resolved by relation-type lookups (served by relations_type_from/to_idx),
// not by property comparison.
func TraversalIndexSpecs(
	prog *predicate.Program, meta *metamodel.Metamodel, fromType string,
) []store.DerivedObjectSpec {
	if prog == nil || meta == nil {
		return nil
	}
	byType := map[string][]string{}
	for _, spec := range prog.Traversals() {
		target, props := traversalIndexTarget(meta, fromType, spec)
		if target == "" || len(props) == 0 {
			continue
		}
		byType[target] = append(byType[target], props...)
	}

	out := make([]store.DerivedObjectSpec, 0, len(byType))
	for target, props := range byType {
		slices.Sort(props)
		out = append(out, store.DerivedObjectSpec{
			Kind: store.DerivedQueryIndex, Type: target, Properties: slices.Compact(props),
		})
	}
	// Deterministic order: the reconciler treats an absent desired object as
	// permission to DROP, so an unstable set would churn indexes between runs.
	slices.SortFunc(out, func(a, b store.DerivedObjectSpec) int {
		return strings.Compare(a.Type, b.Type)
	})
	return out
}

// traversalIndexTarget resolves the final hop's entity type and returns the
// constrained properties that are pushdown-eligible on it.
//
// It returns ("", nil) for anything it cannot resolve — an unknown relation,
// an unresolved union, a property that is not string-shaped. That is the same
// answer as "derives no index", which is the safe direction: a missing index
// costs a scan, whereas an index derived for the wrong shape would be
// reconciled and never used.
func traversalIndexTarget(
	meta *metamodel.Metamodel, fromType string, spec predicate.TraversalSpec,
) (targetType string, props []string) {
	current := fromType
	for i, relType := range spec.Path {
		def, ok := meta.GetRelationDef(relType)
		if !ok {
			return "", nil
		}
		last := i == len(spec.Path)-1
		switch {
		case len(def.To) == 1:
			current = def.To[0]
		case last && spec.EntityType != "" && slices.Contains(def.To, spec.EntityType):
			current = spec.EntityType
		default:
			// A union we cannot resolve. ValidateTraversals refuses these at
			// load, so reaching here means the caller skipped validation;
			// deriving nothing is still the safe answer.
			return "", nil
		}
	}

	for _, name := range spec.PropNames() {
		if !stringComparableOnEveryType(meta, []string{current}, name) {
			continue
		}
		if _, ok := spec.Props[name].(predicate.String); !ok {
			continue
		}
		props = append(props, name)
	}
	if len(props) == 0 {
		return "", nil
	}
	return current, props
}

// staticTraversalSpecs compiles one static query's condition and returns the
// traversal index specs it implies. A condition that does not compile
// contributes nothing, matching [staticIndexProps]: every production entry
// point has already refused such a config through conditionlint.
func staticTraversalSpecs(
	sq *searchparser.SearchQuery, condition string,
	meta *metamodel.Metamodel, ev *predicatefns.Evaluator,
) []store.DerivedObjectSpec {
	if condition == "" || ev == nil || len(sq.EntityTypes) != 1 {
		return nil
	}
	prog, err := ev.CompileWithCurrentUser(sq.EntityTypes[0], condition)
	if err != nil {
		slog.Warn("queryplan: next-action condition skipped for traversal index derivation",
			"type", sq.EntityTypes[0], "error", err)
		return nil
	}
	return TraversalIndexSpecs(prog, meta, sq.EntityTypes[0])
}
