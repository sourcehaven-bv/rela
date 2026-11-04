package dataentryconfig

import (
	"fmt"
	"slices"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/piles"
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
	// Toggles put a button on the entity page that adds the entity to a
	// declared pile, or takes it off again. Favorites are the common case.
	Toggles []PileToggle `yaml:"toggles,omitempty" json:"toggles"`
}

// PileToggle is one entry of `piles.toggles`.
type PileToggle struct {
	// Pile is the key of a pile the schema declares under `piles:`.
	Pile string `yaml:"pile" json:"pile"`
	// Icon is shown while the entity is not on the pile; empty uses the
	// pile's own icon.
	Icon string `yaml:"icon,omitempty" json:"icon,omitempty"`
	// IconActive is shown while the entity is on the pile; empty uses Icon.
	IconActive string `yaml:"icon_active,omitempty" json:"icon_active,omitempty"`
	// Label names the button; empty uses the pile's name.
	Label string `yaml:"label,omitempty" json:"label,omitempty"`
	// EntityTypes limits the button to these types; empty shows it on all.
	EntityTypes []string `yaml:"entity_types,omitempty" json:"entity_types,omitempty"`
}

// pilesKeys are the keys the piles block accepts. The decoder ignores
// unknown nested keys, so a typo such as `action:` would silently offer
// nothing; the block checks its own.
var pilesKeys = []string{"actions", "export", "toggles"}

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
	return append(errs, validatePileToggles(p.Toggles, meta)...)
}

// validatePileToggles checks that every toggle names a declared pile, once,
// and only entity types the schema has.
func validatePileToggles(toggles []PileToggle, meta *metamodel.Metamodel) []string {
	var errs []string
	seen := map[string]bool{}
	for i, tg := range toggles {
		where := fmt.Sprintf("piles.toggles[%d]", i)
		switch {
		case tg.Pile == "":
			errs = append(errs, where+": pile is required")
			continue
		case seen[tg.Pile]:
			errs = append(errs, fmt.Sprintf("%s: pile %q already has a toggle", where, tg.Pile))
		}
		seen[tg.Pile] = true
		for _, icon := range []string{tg.Icon, tg.IconActive} {
			if icon != "" && !slices.Contains(piles.Icons, icon) {
				errs = append(errs, fmt.Sprintf("%s: icon %q is not one of %s", where, icon, strings.Join(piles.Icons, ", ")))
			}
		}
		if meta == nil {
			continue
		}
		if _, ok := meta.Piles[tg.Pile]; !ok {
			errs = append(errs, fmt.Sprintf("%s: pile %q is not declared under the schema's piles:", where, tg.Pile))
		}
		for _, typ := range tg.EntityTypes {
			if _, ok := meta.Entities[typ]; !ok {
				errs = append(errs, fmt.Sprintf("%s: unknown entity type %q", where, typ))
			}
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
