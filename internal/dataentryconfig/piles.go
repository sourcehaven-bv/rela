package dataentryconfig

import (
	"fmt"
	"sort"

	"gopkg.in/yaml.v3"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// PilesConfig is the `piles:` block of data-entry.yaml (TKT-K3RJLH): what a
// pile panel offers. Every key is optional, and so is the block.
//
// Piles exist without it. The block only names what the panel's menu offers
// beyond the built-in rename, delete and copy.
type PilesConfig struct {
	// Actions are ids of global actions the pile menu offers. The SPA runs a
	// chosen action once per item through the ordinary action and PATCH
	// endpoints, so each id must name an action that is not entity-bound
	// (no `available_on:`) and has a label to show.
	Actions []string `yaml:"actions,omitempty" json:"actions"`
	// Export names the registered transforms a pile may be exported with.
	// Empty offers every registered transform.
	Export []string `yaml:"export,omitempty" json:"export"`
}

// pilesKeys are the keys the piles block accepts. The decoder ignores
// unknown nested keys, so a typo such as `action:` would silently offer
// nothing; the block checks its own.
var pilesKeys = []string{"actions", "export"}

// validatePiles checks the `piles:` block: known keys only, every action a
// labeled global action, and every export name a registered transform.
// data is the raw YAML, needed for the unknown-key check.
func validatePiles(data []byte, cfg *Config, meta *metamodel.Metamodel) []string {
	errs := pilesUnknownKeys(data)
	p := cfg.Piles
	if p == nil {
		return errs
	}
	for _, id := range p.Actions {
		action, ok := cfg.Actions[id]
		switch {
		case !ok:
			errs = append(errs, fmt.Sprintf("piles.actions: unknown action %q", id))
		case action.AvailableOn != nil:
			errs = append(errs, fmt.Sprintf(
				"piles.actions: action %q is entity-bound (available_on) and cannot run from a pile", id))
		case action.Label == "":
			errs = append(errs, fmt.Sprintf("piles.actions: action %q needs a label to show in the pile menu", id))
		}
	}
	for _, name := range p.Export {
		if meta == nil {
			break
		}
		if _, ok := meta.Transforms[name]; !ok {
			errs = append(errs, fmt.Sprintf("piles.export: unknown transform %q (not in the schema's transforms:)", name))
		}
	}
	return errs
}

// pilesUnknownKeys reports keys under `piles:` that the block does not
// define.
func pilesUnknownKeys(data []byte) []string {
	var raw struct {
		Piles map[string]any `yaml:"piles"`
	}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil // the struct unmarshal reports malformed YAML
	}
	return unknownKeysIn("piles", raw.Piles, pilesKeys)
}

// pilesSetWarnings flags a `set:` action offered on piles whose properties
// exist on no entity type. A pile holds entities of any type, so the action
// would fail on every item. It is a warning rather than an error because a
// property that some type declares is a legitimate mixed-type action that
// fails only on the other types, which the SPA reports per item.
func pilesSetWarnings(cfg *Config, meta *metamodel.Metamodel) []string {
	if cfg.Piles == nil || meta == nil {
		return nil
	}
	var warnings []string
	for _, id := range cfg.Piles.Actions {
		action, ok := cfg.Actions[id]
		if !ok {
			continue // reported as an error by validatePiles
		}
		props := make([]string, 0, len(action.Set))
		for prop := range action.Set {
			props = append(props, prop)
		}
		sort.Strings(props)
		for _, prop := range props {
			if !anyTypeDeclares(meta, prop) {
				warnings = append(warnings, fmt.Sprintf(
					"piles.actions: action %q sets property %q, which no entity type declares; "+
						"it will fail on every pile item", id, prop))
			}
		}
	}
	return warnings
}

// anyTypeDeclares reports whether some entity type declares prop.
func anyTypeDeclares(meta *metamodel.Metamodel, prop string) bool {
	for _, def := range meta.Entities {
		if _, ok := def.Properties[prop]; ok {
			return true
		}
	}
	return false
}
