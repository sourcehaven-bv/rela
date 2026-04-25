package metamodel

import "fmt"

// GetPlural returns the plural label for an entity type
func (e *EntityDef) GetPlural() string {
	if e.LabelPlural != "" {
		return e.LabelPlural
	}
	return e.Label + "s"
}

// GetDirPlural returns the plural form to use for directory names
func (e *EntityDef) GetDirPlural(typeName string) string {
	if e.Plural != "" {
		return e.Plural
	}
	// Fall back to naive pluralization of the type name
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
func (e *EntityDef) GetPrimaryProperty() string {
	// (1) Author-declared override wins.
	if e.DisplayProperty != "" {
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
func (e *EntityDef) DisplayTitle(id string, properties map[string]interface{}) string {
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
