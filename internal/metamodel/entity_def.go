package metamodel

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

// GetPrimaryProperty returns the name of the primary required string property.
// This is typically "title" or "name" - the first required string property found.
// Returns empty string if no suitable property exists.
func (e *EntityDef) GetPrimaryProperty() string {
	// Check common names first in priority order
	priorityNames := []string{"title", "name", "label"}
	for _, name := range priorityNames {
		if prop, ok := e.Properties[name]; ok {
			if prop.Required && (prop.Type == PropertyTypeString || prop.Type == "") {
				return name
			}
		}
	}

	// Fall back to finding any required string property (sorted for determinism)
	var candidates []string
	for name, prop := range e.Properties {
		if prop.Required && (prop.Type == PropertyTypeString || prop.Type == "") {
			candidates = append(candidates, name)
		}
	}
	if len(candidates) > 0 {
		// Sort for deterministic behavior
		for i := 1; i < len(candidates); i++ {
			for j := i; j > 0 && candidates[j] < candidates[j-1]; j-- {
				candidates[j], candidates[j-1] = candidates[j-1], candidates[j]
			}
		}
		return candidates[0]
	}

	return ""
}

// GetIDType returns the ID type for this entity, defaulting to "auto".
func (e *EntityDef) GetIDType() string {
	if e.IDType == "" {
		return IDTypeAuto
	}
	return e.IDType
}

// IsAutoID returns true if this entity type uses auto-generated IDs
func (e *EntityDef) IsAutoID() bool {
	return e.GetIDType() == IDTypeAuto
}

// IsManualID returns true if this entity type uses manually-specified IDs
func (e *EntityDef) IsManualID() bool {
	return e.GetIDType() == IDTypeManual
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
