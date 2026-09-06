// Package queryplan compiles the safe store-side subset of data-entry search
// queries. Runtime pushdown and static index planning share this package.
package queryplan

import (
	"slices"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	"github.com/Sourcehaven-BV/rela/internal/filter"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/predicate"
	"github.com/Sourcehaven-BV/rela/internal/predicatefns"
	"github.com/Sourcehaven-BV/rela/internal/search/searchparser"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// LoadStaticIndexSpecs parses and validates a complete data-entry config before
// deriving specs. An error yields no partial desired set: reconciliation treats
// absence as permission to drop owned indexes.
func LoadStaticIndexSpecs(data []byte, meta *metamodel.Metamodel) ([]store.DerivedObjectSpec, error) {
	var cfg dataentryconfig.Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	if err := dataentryconfig.ValidateConfig(data, &cfg, meta); err != nil {
		return nil, err
	}
	return StaticIndexSpecs(&cfg, meta), nil
}

// PushdownPrefilters returns the store-evaluable pre-filter subset. The caller
// must still run every filter through the metamodel-aware evaluator.
func PushdownPrefilters(filters []*filter.Filter, meta *metamodel.Metamodel, types []string) []store.PropPredicate {
	if len(types) == 0 || meta == nil {
		return nil
	}
	var pushed []store.PropPredicate
	for _, f := range filters {
		if f.IsGlob || f.Operator != filter.OpEqual || !stringComparableOnEveryType(meta, types, f.Property) {
			continue
		}
		pushed = append(pushed, store.PropPredicate{
			Property: f.Property, Op: store.PropEqual, Value: f.Value, Scalar: f.Value != "",
		})
	}
	return pushed
}

func stringComparableOnEveryType(meta *metamodel.Metamodel, types []string, prop string) bool {
	return declaredOnEveryType(meta, types, prop, false)
}

// stringListOnEveryType is the list twin of stringComparableOnEveryType: the
// property must be a LIST of strings on every type, so a membership predicate
// compares element text the same way the Go pass does.
func stringListOnEveryType(meta *metamodel.Metamodel, types []string, prop string) bool {
	return declaredOnEveryType(meta, types, prop, true)
}

func declaredOnEveryType(meta *metamodel.Metamodel, types []string, prop string, list bool) bool {
	if len(types) == 0 {
		return false
	}
	for _, typ := range types {
		def, ok := meta.GetEntityDef(typ)
		if !ok {
			return false
		}
		pd, ok := def.Properties[prop]
		if !ok || pd.List != list || pd.Type != metamodel.PropertyTypeString {
			return false
		}
	}
	return true
}

// StaticIndexSpecs derives one composite index per canonical static query
// shape. Query literal values are deliberately absent from the spec.
func StaticIndexSpecs(cfg *dataentryconfig.Config, meta *metamodel.Metamodel) []store.DerivedObjectSpec {
	if cfg == nil || meta == nil {
		return nil
	}
	var ev *predicatefns.Evaluator
	byKey := make(map[string]store.DerivedObjectSpec)
	for _, q := range staticQueries(cfg) {
		sq := searchparser.ParseQuery(q.query)
		if len(sq.ParseErrors) != 0 || len(sq.EntityTypes) != 1 || sq.HasFreeText() {
			continue
		}
		if q.condition != "" && ev == nil {
			ev = predicatefns.NewEvaluator(meta)
		}
		props := staticIndexProps(sq, q.condition, meta, ev)
		if len(props) == 0 {
			continue
		}
		spec := store.DerivedObjectSpec{Kind: store.DerivedQueryIndex, Type: sq.EntityTypes[0], Properties: props}
		byKey[spec.Type+"\x00"+strings.Join(props, "\x00")] = spec
	}
	keys := make([]string, 0, len(byKey))
	for key := range byKey {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	out := make([]store.DerivedObjectSpec, 0, len(keys))
	for _, key := range keys {
		out = append(out, byKey[key])
	}
	return out
}

// staticQuery is one operator-declared query and, for a next-action source,
// the condition that is pushed down beside it at runtime. The two are ANDed
// by the store, so they share ONE composite index — the same shape the
// runtime pushdown (dataentry's executeQuery with the condition prefilters
// appended) actually probes.
type staticQuery struct{ query, condition string }

// staticQueries collects every query shape the config declares statically.
func staticQueries(cfg *dataentryconfig.Config) []staticQuery {
	var queries []staticQuery
	if cfg.Dashboard != nil {
		for _, card := range cfg.Dashboard.Cards {
			queries = append(queries, staticQuery{query: card.Query})
		}
	}
	for _, src := range cfg.NextActions {
		if src.Query != "" {
			queries = append(queries, staticQuery{query: src.Query, condition: src.Condition})
		}
		for _, offer := range src.Actions {
			if offer.PickOne != nil {
				queries = append(queries, staticQuery{query: offer.PickOne.Query})
			}
		}
	}
	return queries
}

// staticIndexProps returns the sorted, deduplicated index columns for one
// static query: the query's scalar pushdown properties plus, when a condition
// is present, its scalar pushdown properties ([ConditionIndexProperties]).
//
// A condition that does not compile contributes nothing, and that is not a
// partial-set hazard: conditionlint refuses the same expression at load, so
// no runtime query will ever probe for the index it would have described.
func staticIndexProps(
	sq *searchparser.SearchQuery, condition string, meta *metamodel.Metamodel, ev *predicatefns.Evaluator,
) []string {
	pushed := PushdownPrefilters(sq.PropertyFilters, meta, sq.EntityTypes)
	props := make([]string, 0, len(pushed))
	for _, p := range pushed {
		if p.Scalar {
			props = append(props, p.Property)
		}
	}
	if condition != "" && ev != nil {
		if prog, err := ev.CompileWithCurrentUser(sq.EntityTypes[0], condition); err == nil {
			props = append(props, ConditionIndexProperties(prog, meta, sq.EntityTypes)...)
		}
	}
	slices.Sort(props)
	return slices.Compact(props)
}

// ConditionPrefilters returns the store-evaluable pre-filter subset of a
// compiled predicate condition, resolving the current user's identity to
// a literal.
//
// It is the predicate-path twin of [PushdownPrefilters] and carries the
// identical contract: the returned predicates are a PRE-FILTER only, and
// the caller must still evaluate the whole program in Go. The store may
// remove rows the Go pass would also have removed — never more.
//
// Soundness rests on two independent gates:
//
//   - [predicate.Program.ConstEqualities] restricts the shape to
//     top-level ANDed equalities against a request-constant (a literal,
//     current_user.id, or the is_current_user / has_current_user sugar),
//     so a pushed predicate can never contradict the program.
//   - The metamodel gate (shared with the filter path) restricts it to
//     declared string properties — scalar for an equality, list-of-string
//     for a membership — so the store's string-form comparison cannot
//     disagree with the metamodel-aware Go pass on a typed value.
//
// A membership (`has_current_user(entity.watchers)`) lowers to a
// NON-scalar [store.PropEqual], which every backend already defines as
// "some element equals" for a list value (the multi-select rule pinned by
// storetest's Props_value_shapes). It needs no new store operator, but it
// is not indexable by the derived static-query index, which covers scalar
// text only — see [ConditionIndexProperties].
//
// `identity` is the current user's query identity (see
// predicatefns.QueryIdentity.ID). An EMPTY identity pushes nothing
// rather than an empty-string equality: an unidentified request must not
// silently pre-filter to the rows whose property is unset. The Go pass
// fails that request closed on its own; this must not quietly answer it
// first.
func ConditionPrefilters(
	prog *predicate.Program, meta *metamodel.Metamodel, types []string, identity string,
) []store.PropPredicate {
	var pushed []store.PropPredicate
	for _, eq := range conditionEqualities(prog, meta, types) {
		value := eq.Value
		if eq.FromVar != "" {
			if identity == "" {
				continue
			}
			value = identity
		}
		pushed = append(pushed, store.PropPredicate{
			Property: eq.Attribute, Op: store.PropEqual, Value: value, Scalar: !eq.List && value != "",
		})
	}
	return pushed
}

// ConditionIndexProperties returns the properties of a compiled condition
// that [ConditionPrefilters] would push as SCALAR equalities — the shape
// the derived static-query index covers — regardless of what the identity
// resolves to at request time.
//
// It is the index-inference half of the eligibility decision
// ConditionPrefilters makes at runtime, and the two must not drift: an
// index derived for a predicate that is never pushed is dead weight, and
// a pushed predicate with no index is a sequential scan the operator was
// promised would not happen. Both call conditionEqualities, so they
// cannot disagree about which conjuncts qualify. Memberships are
// excluded here because the composite btree over `properties ->> p` does
// not serve a jsonb containment probe.
func ConditionIndexProperties(prog *predicate.Program, meta *metamodel.Metamodel, types []string) []string {
	var props []string
	for _, eq := range conditionEqualities(prog, meta, types) {
		if eq.List {
			continue
		}
		props = append(props, eq.Attribute)
	}
	return props
}

// conditionEqualities is the shared eligibility core: the program's
// request-constant equalities that ALSO pass the metamodel gate for every
// type they will be evaluated against. Only the identity field of
// current_user resolves — `tool` is deliberately not pushable: it is
// diagnostic, never an authorization or membership input (see
// internal/affordances), and pushing it would invite exactly that use.
func conditionEqualities(
	prog *predicate.Program, meta *metamodel.Metamodel, types []string,
) []predicate.ConstEquality {
	if prog == nil || meta == nil || len(types) == 0 {
		return nil
	}
	var out []predicate.ConstEquality
	for _, eq := range prog.ConstEqualities(predicatefns.CurrentUserPrefilterSpec()) {
		if eq.FromVar != "" && eq.FromVar != predicatefns.FieldCurrentUserID {
			continue
		}
		if eq.List {
			if !stringListOnEveryType(meta, types, eq.Attribute) {
				continue
			}
		} else if !stringComparableOnEveryType(meta, types, eq.Attribute) {
			continue
		}
		out = append(out, eq)
	}
	return out
}
