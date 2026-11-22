package dataentryconfig

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/Sourcehaven-BV/rela/internal/yamlscalar"
)

// ChromeToggle switches a built-in element of the SPA on or off, such as the
// dashboard's validation card or the History button of an entity page header
// (TKT-31822PN). In YAML it is written in one of three ways:
//
//	validation: true                      # shown (the default)
//	validation: false                     # never shown
//	validation: {permission: staff.read}  # shown to holders of the permission
//
// The zero value is "shown", so a config that does not name the element
// keeps the behavior it had before the key existed.
//
// The permission form has the semantics of [NavigationEntry.Permission]: a UX
// filter, NOT access control. Hiding the History button does not stop
// anyone reading `/api/v1/_history`, which keeps its own ACL, and the toggle
// itself is served to every principal on `/api/v1/_config` (see "The
// configuration is not a secret; the data is" in the root CLAUDE.md). With no
// `acl.yaml`, and under `--read-only`, a gated element is shown.
type ChromeToggle struct {
	// Off is true for `false`: the element is shown to nobody.
	Off bool
	// Permission names the global ACL permission that shows the element.
	// Empty means no gate.
	Permission string
}

// chromeToggleKeys are the keys the mapping form accepts.
var chromeToggleKeys = []string{"permission"}

// errChromeToggleShape is the error for a value that is neither a boolean nor
// a mapping with `permission:`.
var errChromeToggleShape = errors.New("must be true, false or {permission: <name>}")

// Shown reports whether the element is shown, given a check of whether the
// principal may see an element gated on a permission. permits is called only
// for the permission form. Nil: must not be nil when Permission is set (it
// would panic); not called otherwise.
func (t ChromeToggle) Shown(permits func(permission string) bool) bool {
	if t.Off {
		return false
	}
	if t.Permission == "" {
		return true
	}
	return permits(t.Permission)
}

// IsZero reports whether the toggle is the default, so `omitzero` leaves an
// unset toggle out of the JSON.
func (t ChromeToggle) IsZero() bool {
	return !t.Off && t.Permission == ""
}

// UnmarshalYAML reads `true`, `false` or `{permission: <name>}`. yaml.v3
// adds no position to an error from a custom unmarshaler, so each error names
// the line, which tells the operator which toggle is wrong.
func (t *ChromeToggle) UnmarshalYAML(value *yaml.Node) error {
	if err := t.unmarshalYAML(value); err != nil {
		return fmt.Errorf("line %d: %w", value.Line, err)
	}
	return nil
}

func (t *ChromeToggle) unmarshalYAML(value *yaml.Node) error {
	switch value.Kind {
	case yaml.ScalarNode:
		// YAML 1.2 reads an unquoted yes/no/on/off as a string; name the
		// mistake rather than the generic shape error.
		if yamlscalar.PlainYAML11Bool(value) {
			return fmt.Errorf("%q is not a boolean; use true or false", strings.TrimSpace(value.Value))
		}
		if value.Tag != "!!bool" {
			return errChromeToggleShape
		}
		var b bool
		if err := value.Decode(&b); err != nil {
			return err
		}
		*t = ChromeToggle{Off: !b}
		return nil
	case yaml.MappingNode:
		var permission string
		for i := 0; i+1 < len(value.Content); i += 2 {
			key, val := value.Content[i], value.Content[i+1]
			if key.Value != "permission" {
				return fmt.Errorf("unknown key %q (valid keys: %s)", key.Value, strings.Join(chromeToggleKeys, ", "))
			}
			if val.Kind != yaml.ScalarNode || val.Tag != "!!str" || val.Value == "" ||
				strings.TrimSpace(val.Value) != val.Value {

				return errors.New("permission must be a permission name without surrounding spaces")
			}
			permission = val.Value
		}
		if permission == "" {
			return errors.New("permission is required in the mapping form; use true or false for no gate")
		}
		*t = ChromeToggle{Permission: permission}
		return nil
	default:
		return errChromeToggleShape
	}
}

// MarshalYAML writes the shape UnmarshalYAML reads.
func (t ChromeToggle) MarshalYAML() (any, error) {
	return t.wire(), nil
}

// MarshalJSON writes the same shape as YAML: true, false or
// {"permission": name}. `/api/v1/_config` serves it verbatim, as it serves a
// card's `permission:`.
func (t ChromeToggle) MarshalJSON() ([]byte, error) {
	return json.Marshal(t.wire())
}

// UnmarshalJSON reads what MarshalJSON writes. A JSON null leaves the
// toggle at its default, as an absent YAML value does.
func (t *ChromeToggle) UnmarshalJSON(data []byte) error {
	if string(bytes.TrimSpace(data)) == "null" {
		return nil
	}
	var b bool
	if err := json.Unmarshal(data, &b); err == nil {
		*t = ChromeToggle{Off: !b}
		return nil
	}
	var m struct {
		Permission string `json:"permission"`
	}
	if err := json.Unmarshal(data, &m); err != nil || m.Permission == "" {
		return errChromeToggleShape
	}
	*t = ChromeToggle{Permission: m.Permission}
	return nil
}

func (t ChromeToggle) wire() any {
	switch {
	case t.Off:
		return false
	case t.Permission != "":
		return map[string]string{"permission": t.Permission}
	default:
		return true
	}
}

// HeaderButtonsConfig is the `header_buttons:` block of data-entry.yaml: which
// built-in buttons the header of an entity page shows (TKT-31822PN). Every
// key is optional, and so is the block; an absent key shows its button as
// before.
//
// A hidden button is a UX choice, not access control; see [ChromeToggle].
type HeaderButtonsConfig struct {
	// History is the History button. It shows only where the deployment
	// keeps version history at all (`app.history_enabled` on /_config); this
	// toggle can only hide it further.
	History ChromeToggle `yaml:"history,omitempty" json:"history,omitzero"`
	// AddToPile is the "Add to pile" menu, on the desktop row and in the
	// overflow menu. It shows only where piles are available to the
	// principal; this toggle can only hide it further. The pile toggles of
	// `piles.toggles` and "Remove from pile" while stepping through a pile
	// are not affected.
	AddToPile ChromeToggle `yaml:"add_to_pile,omitempty" json:"add_to_pile,omitzero"`
}

// headerButtonsKeys are the keys the header_buttons block accepts.
var headerButtonsKeys = []string{"history", "add_to_pile"}

// headerButtonsUnknownKeys reports keys under `header_buttons:` that the block
// does not define. The decoder ignores unknown nested keys, so a typo such as
// `histroy: false` would silently hide nothing.
func headerButtonsUnknownKeys(data []byte) []string {
	var raw struct {
		HeaderButtons map[string]any `yaml:"header_buttons"`
	}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil // the struct unmarshal reports malformed YAML
	}
	return unknownKeysIn("header_buttons", raw.HeaderButtons, headerButtonsKeys)
}

// ListButtonsConfig is the `list_buttons:` block of data-entry.yaml: which
// built-in buttons lists and search results offer. Every key is optional, and
// so is the block; an absent key shows its button as before.
//
// A hidden button is a UX choice, not access control; see [ChromeToggle].
type ListButtonsConfig struct {
	// AddToPile is "Add to pile" in the bulk bar of a list and on search
	// results. It shows only where piles are available to the principal;
	// this toggle can only hide it further. A list without it, without a
	// bulk action and without a row the reader may delete shows no
	// selection checkboxes.
	AddToPile ChromeToggle `yaml:"add_to_pile,omitempty" json:"add_to_pile,omitzero"`
}

// PageChrome is the part of [Config] that hides built-in buttons and panels
// of the SPA. Its zero value shows them all.
type PageChrome struct {
	// HeaderButtons hides built-in buttons of the entity page header. See
	// [HeaderButtonsConfig].
	HeaderButtons HeaderButtonsConfig `yaml:"header_buttons,omitempty" json:"header_buttons,omitzero"`

	// ListButtons hides built-in buttons of lists and search results. See
	// [ListButtonsConfig].
	ListButtons ListButtonsConfig `yaml:"list_buttons,omitempty" json:"list_buttons,omitzero"`

	// EntityPanels hides built-in panels of the entity page, for every
	// space. See [EntityPanelsConfig].
	EntityPanels EntityPanelsConfig `yaml:"entity_panels,omitempty" json:"entity_panels,omitzero"`
}

// EntityPanelsConfig is the `entity_panels:` block of data-entry.yaml, at
// the top level and on a space: which built-in panels an entity page shows.
// Every key is optional, and so is the block; an absent key shows its panel
// as before.
//
// Unlike the buttons, a hidden panel is also withheld by the server: the
// route that feeds it answers as if there were nothing to show. The data
// behind it keeps its own ACL and stays reachable by its other routes.
type EntityPanelsConfig struct {
	// FlowRuns is the panel that lists the flow runs on the entity and
	// offers the flows the reader may start on it. Hidden, the subject runs
	// route (`GET /api/v1/_flows/runs?subject=`) answers no runs and no
	// startable flows.
	FlowRuns ChromeToggle `yaml:"flow_runs,omitempty" json:"flow_runs,omitzero"`
}

// entityPanelsKeys are the keys the entity_panels block accepts.
var entityPanelsKeys = []string{"flow_runs"}

// entityPanelsUnknownKeys is [headerButtonsUnknownKeys] for `entity_panels:`,
// at the top level and on each space.
func entityPanelsUnknownKeys(data []byte) []string {
	var raw struct {
		EntityPanels map[string]any `yaml:"entity_panels"`
		Spaces       []struct {
			ID           string         `yaml:"id"`
			EntityPanels map[string]any `yaml:"entity_panels"`
		} `yaml:"spaces"`
	}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil // the struct unmarshal reports malformed YAML
	}
	errs := unknownKeysIn("entity_panels", raw.EntityPanels, entityPanelsKeys)
	for _, sp := range raw.Spaces {
		errs = append(errs, unknownKeysIn(fmt.Sprintf("spaces[%s].entity_panels", sp.ID), sp.EntityPanels,
			entityPanelsKeys)...)
	}
	return errs
}

// FlowRunsPanelShown reports whether an entity page in space shows the flow
// runs panel: neither the top-level `entity_panels.flow_runs` nor that of
// space hides it. An empty or unknown space applies the top-level toggle
// alone. permits answers whether the principal holds a permission, as for
// [ChromeToggle.Shown].
func (c *Config) FlowRunsPanelShown(space string, permits func(permission string) bool) bool {
	if c == nil {
		return true
	}
	if !c.EntityPanels.FlowRuns.Shown(permits) {
		return false
	}
	for _, sp := range c.Spaces {
		if sp.ID == space {
			return sp.EntityPanels.FlowRuns.Shown(permits)
		}
	}
	return true
}

// listButtonsKeys are the keys the list_buttons block accepts.
var listButtonsKeys = []string{"add_to_pile"}

// listButtonsUnknownKeys is [headerButtonsUnknownKeys] for `list_buttons:`.
func listButtonsUnknownKeys(data []byte) []string {
	var raw struct {
		ListButtons map[string]any `yaml:"list_buttons"`
	}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil // the struct unmarshal reports malformed YAML
	}
	return unknownKeysIn("list_buttons", raw.ListButtons, listButtonsKeys)
}

// validateDashboardValidation refuses `validation: false` or a permission on
// an entity dashboard. Such a dashboard never shows the validation card,
// because the card covers the whole project and an entity dashboard is about
// one entity, so the setting could only mislead the reader of the config.
// `validation: true` is the default and is accepted, as an omitted key is.
func validateDashboardValidation(prefix string, d *DashboardConfig) []string {
	if d.IsEntityDashboard() && !d.Validation.IsZero() {
		return []string{prefix + ": validation has no effect on an entity dashboard, " +
			"which never shows the validation card; remove it"}
	}
	return nil
}
