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
	for _, typ := range types {
		def, ok := meta.GetEntityDef(typ)
		if !ok {
			return false
		}
		pd, ok := def.Properties[prop]
		if !ok || pd.List || pd.Type != metamodel.PropertyTypeString {
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
	var queries []string
	if cfg.Dashboard != nil {
		for _, card := range cfg.Dashboard.Cards {
			queries = append(queries, card.Query)
		}
	}
	for _, src := range cfg.NextActions {
		if src.Query != "" {
			queries = append(queries, src.Query)
		}
		for _, offer := range src.Actions {
			if offer.PickOne != nil {
				queries = append(queries, offer.PickOne.Query)
			}
		}
	}

	byKey := make(map[string]store.DerivedObjectSpec)
	for _, raw := range queries {
		sq := searchparser.ParseQuery(raw)
		if len(sq.ParseErrors) != 0 || len(sq.EntityTypes) != 1 || sq.HasFreeText() {
			continue
		}
		pushed := PushdownPrefilters(sq.PropertyFilters, meta, sq.EntityTypes)
		props := make([]string, 0, len(pushed))
		for _, p := range pushed {
			if p.Scalar {
				props = append(props, p.Property)
			}
		}
		slices.Sort(props)
		props = slices.Compact(props)
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
