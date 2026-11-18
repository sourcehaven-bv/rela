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
// dashboard's validation card or the History button of an entity header
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

// EntityHeaderConfig is the `entity_header:` block of data-entry.yaml: which
// built-in buttons the header of an entity page shows (TKT-31822PN). Every
// key is optional, and so is the block; an absent key shows its button as
// before.
//
// A hidden button is a UX choice, not access control; see [ChromeToggle].
type EntityHeaderConfig struct {
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

// entityHeaderKeys are the keys the entity_header block accepts.
var entityHeaderKeys = []string{"history", "add_to_pile"}

// entityHeaderUnknownKeys reports keys under `entity_header:` that the block
// does not define. The decoder ignores unknown nested keys, so a typo such as
// `histroy: false` would silently hide nothing.
func entityHeaderUnknownKeys(data []byte) []string {
	var raw struct {
		EntityHeader map[string]any `yaml:"entity_header"`
	}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil // the struct unmarshal reports malformed YAML
	}
	return unknownKeysIn("entity_header", raw.EntityHeader, entityHeaderKeys)
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
