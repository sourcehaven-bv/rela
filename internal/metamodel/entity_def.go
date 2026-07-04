package metamodel

import (
	"fmt"
	"strings"
)

// isDisplayTemplate reports whether a display_property value is a template
// (contains a '{' placeholder) rather than a bare property name. Property
// names can't contain '{' in a valid metamodel, so the presence of one
// disambiguates cleanly.
func isDisplayTemplate(displayProperty string) bool {
	return strings.ContainsRune(displayProperty, '{')
}

// parseDisplayTemplate scans a display_property template and returns the
// property names referenced by its `{name}` placeholders, in order. It
// returns an error for a malformed template: an unclosed `{`, or an empty
// placeholder `{}`. Literal text between placeholders is ignored here — it
// only matters at render time.
//
// This is the single source of truth for template syntax, shared by
// renderDisplayTemplate (runtime) and validateDisplayProperty (load time).
func parseDisplayTemplate(tmpl string) ([]string, error) {
	var names []string
	for i := 0; i < len(tmpl); i++ {
		if tmpl[i] != '{' {
			continue
		}
		end := strings.IndexByte(tmpl[i:], '}')
		if end < 0 {
			return nil, fmt.Errorf("unclosed '{' in template %q", tmpl)
		}
		name := tmpl[i+1 : i+end]
		if name == "" {
			return nil, fmt.Errorf("empty placeholder '{}' in template %q", tmpl)
		}
		names = append(names, name)
		i += end
	}
	return names, nil
}

// renderDisplayTemplate substitutes each `{name}` placeholder with the
// stringified property value, passes literal text through, then collapses
// consecutive whitespace to a single space and trims the result. A nil or
// missing property renders as empty, so `"{a} {b}"` with an empty middle
// field collapses to a single space rather than a double. The caller
// (DisplayTitle) falls back to the entity ID when the result is empty.
//
// The template is assumed already validated at load time (parseDisplayTemplate
// succeeded and every placeholder names a defined property), so a malformed
// template here degrades gracefully rather than erroring: an unclosed '{' is
// emitted verbatim.
func renderDisplayTemplate(tmpl string, properties map[string]interface{}) string {
	var b strings.Builder
	for i := 0; i < len(tmpl); i++ {
		if tmpl[i] != '{' {
			b.WriteByte(tmpl[i])
			continue
		}
		end := strings.IndexByte(tmpl[i:], '}')
		if end < 0 {
			// Malformed (shouldn't happen post-validation); emit verbatim.
			b.WriteString(tmpl[i:])
			break
		}
		name := tmpl[i+1 : i+end]
		if val, ok := properties[name]; ok && val != nil {
			fmt.Fprintf(&b, "%v", val)
		}
		i += end
	}
	return collapseWhitespace(b.String())
}

// collapseWhitespace replaces every run of whitespace with a single space and
// trims the ends. Newlines are treated as ordinary whitespace (multi-line
// display titles are out of scope).
func collapseWhitespace(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

// GetLabelPlural returns the human-readable plural label for an entity
// type (used in UI strings, e.g. "List of Features").
func (e *EntityDef) GetLabelPlural() string {
	if e.LabelPlural != "" {
		return e.LabelPlural
	}
	return e.Label + "s"
}

// GetPlural returns the slug-form plural for an entity type (used as
// URL segments, fsstore directory names, OpenAPI paths). Falls back to
// naive pluralization of the type name when not explicitly set.
func (e *EntityDef) GetPlural(typeName string) string {
	if e.Plural != "" {
		return e.Plural
	}
	return typeName + "s"
}

// GetDefaultStatus returns the default status value for this entity type.
// It checks the entity's status property definition for a custom type or inline values.
// If no explicit default exists, returns the first valid value, or "draft" as final fallback.
func (e *EntityDef) GetDefaultStatus(m *Metamodel) string {
	statusProp, ok := e.Properties["status"]
	if !ok {
		// No status property defined, use standard default
		return "draft"
	}

	// Check for explicit default in property definition
	if statusProp.Default != "" {
		return statusProp.Default
	}

	// Check for inline enum values
	if len(statusProp.Values) > 0 {
		return statusProp.Values[0]
	}

	// Check for custom type
	if statusProp.Type != "" && statusProp.Type != "status" && statusProp.Type != "string" {
		if customType, ok := m.Types[statusProp.Type]; ok {
			if customType.Default != "" {
				return customType.Default
			}
			if len(customType.Values) > 0 {
				return customType.Values[0]
			}
		}
	}

	// Standard "status" type - use "draft" as default
	return "draft"
}

// GetPrimaryProperty returns the name of the primary property used as
// the entity's display name.
//
// Resolution order:
//
//  1. Explicit `display_property` set on the entity definition. The
//     name is returned verbatim — load-time validation already
//     guaranteed it references a defined property.
//  2. The first match in the priority list `title`/`name`/`label`,
//     when defined as a required string.
//  3. Any required string property (alphabetical for determinism).
//  4. Empty string when no candidate exists.
//
// A *templated* display_property (containing `{`) has no single primary
// property — it is a readonly, derived display string. GetPrimaryProperty
// returns "" for it; DisplayTitle renders the template directly. Callers
// that treat the result as a writable property key (e.g. a create title
// shortcut) therefore get "" and skip, which is correct: there is no single
// field to write into.
func (e *EntityDef) GetPrimaryProperty() string {
	// (1) Author-declared override wins — unless it's a template, which
	// names no single property.
	if e.DisplayProperty != "" {
		if isDisplayTemplate(e.DisplayProperty) {
			return ""
		}
		return e.DisplayProperty
	}

	// (2) Priority list of conventional names.
	priorityNames := []string{"title", "name", "label"}
	for _, name := range priorityNames {
		if prop, ok := e.Properties[name]; ok {
			if prop.Required && (prop.Type == PropertyTypeString || prop.Type == "") {
				return name
			}
		}
	}

	// (3) Fall back to finding any required string property
	// (sorted for determinism).
	var candidates []string
	for name, prop := range e.Properties {
		if prop.Required && (prop.Type == PropertyTypeString || prop.Type == "") {
			candidates = append(candidates, name)
		}
	}
	if len(candidates) > 0 {
		for i := 1; i < len(candidates); i++ {
			for j := i; j > 0 && candidates[j] < candidates[j-1]; j-- {
				candidates[j], candidates[j-1] = candidates[j-1], candidates[j]
			}
		}
		return candidates[0]
	}

	return ""
}

// DisplayTitle returns the display title for an entity using its
// type's primary property. Behavior:
//
//   - String value: returned verbatim (the common case).
//   - Non-string value (number, boolean, enum stored as a typed value):
//     stringified via fmt.Sprintf("%v", val) so an explicit
//     display_property: status (an enum) shows the value and not the
//     ID. nil values fall through to the ID — `%v` on nil yields
//     "<nil>" which would be a worse display name than the ID.
//   - Missing or empty-after-stringification: falls back to the ID.
//
// The non-string stringification is what makes the explicit
// display_property override pay off for enum-typed fields. See
// review-response RR-9CW5N.
//
// When display_property is a template (contains `{`), the placeholders are
// substituted from properties, whitespace is collapsed, and the result is
// returned — falling back to the ID when it renders empty.
func (e *EntityDef) DisplayTitle(id string, properties map[string]interface{}) string {
	if isDisplayTemplate(e.DisplayProperty) {
		if s := renderDisplayTemplate(e.DisplayProperty, properties); s != "" {
			return s
		}
		return id
	}

	primary := e.GetPrimaryProperty()
	if primary == "" {
		return id
	}
	val, ok := properties[primary]
	if !ok {
		return id
	}
	if s, ok := val.(string); ok {
		if s != "" {
			return s
		}
		return id
	}
	if val == nil {
		return id
	}
	if s := fmt.Sprintf("%v", val); s != "" {
		return s
	}
	return id
}

// GetIDType returns the ID type for this entity, defaulting to "short".
func (e *EntityDef) GetIDType() string {
	if e.IDType == "" {
		return IDTypeShort
	}
	return e.IDType
}

// IsShortID returns true if this entity type uses short random IDs
func (e *EntityDef) IsShortID() bool {
	return e.GetIDType() == IDTypeShort
}

// IsSequentialID returns true if this entity type uses auto-generated sequential IDs
func (e *EntityDef) IsSequentialID() bool {
	return e.GetIDType() == IDTypeSequential
}

// IsManualID returns true if this entity type uses manually-specified IDs
func (e *EntityDef) IsManualID() bool {
	return e.GetIDType() == IDTypeManual
}

// GetIDCaps returns the ID capitalization mode for short IDs, defaulting to "upper".
func (e *EntityDef) GetIDCaps() string {
	if e.IDCaps == "" {
		return IDCapsUpper
	}
	return e.IDCaps
}

// GetIDPrefixes returns the effective ID prefixes for this entity type.
// It normalizes id_prefix (singular) and id_prefixes (plural) into a single list.
func (e *EntityDef) GetIDPrefixes() []string {
	// If id_prefix is set (singular), return it as a single-element slice
	if e.IDPrefix != "" {
		return []string{e.IDPrefix}
	}
	// If id_prefixes is set (plural), return it
	return e.IDPrefixes
}

// HasPattern checks if the entity type matches a given ID pattern
func (e *EntityDef) HasPattern(pattern string) bool {
	for _, p := range e.GetIDPrefixes() {
		if p == pattern {
			return true
		}
	}
	return false
}

// MatchesID checks if an ID matches any of this entity type's prefixes
func (e *EntityDef) MatchesID(id string) bool {
	for _, prefix := range e.GetIDPrefixes() {
		if len(id) >= len(prefix) && id[:len(prefix)] == prefix {
			return true
		}
	}
	return false
}

// GetPropertyOrder returns the property names in their definition order.
// If PropertyOrder was not populated during loading, returns nil.
// Returns a copy to prevent external modification.
func (e *EntityDef) GetPropertyOrder() []string {
	if e.PropertyOrder == nil {
		return nil
	}
	result := make([]string, len(e.PropertyOrder))
	copy(result, e.PropertyOrder)
	return result
}
