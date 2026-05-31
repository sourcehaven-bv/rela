package mcp

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

func (s *Server) resolveType(typeName string) string {
	typeName = strings.TrimSpace(typeName)
	meta := s.deps.Meta
	resolved := meta.ResolveAlias(typeName)
	if _, ok := meta.GetEntityDef(resolved); ok {
		return resolved
	}
	// Try stripping plural
	for _, suffix := range []string{"ies", "es", "s"} {
		replacements := map[string]string{"ies": "y", "es": "", "s": ""}
		if strings.HasSuffix(typeName, suffix) {
			singular := strings.TrimSuffix(typeName, suffix) + replacements[suffix]
			resolved = meta.ResolveAlias(singular)
			if _, ok := meta.GetEntityDef(resolved); ok {
				return resolved
			}
		}
	}
	return typeName
}

// trimID trims whitespace from an entity ID
func trimID(id string) string {
	return strings.TrimSpace(id)
}

func (s *Server) resolveEntityType(typeName string) (string, *metamodel.EntityDef, error) {
	resolved := s.resolveType(typeName)
	def, ok := s.deps.Meta.GetEntityDef(resolved)
	if !ok {
		return "", nil, fmt.Errorf("unknown entity type: %s", typeName)
	}
	return resolved, def, nil
}

// extractProperties parses the `properties` argument and filters out both nil
// and empty-string entries (both treated as "no value"). Used by create paths.
func extractProperties(request mcp.CallToolRequest) map[string]interface{} {
	props, ok := parsePropertiesArg(request)
	if !ok {
		return nil
	}
	return filterProperties(props, false)
}

// extractPropertiesAllowNil parses the `properties` argument and preserves nil
// entries so update_entity can use them as a delete sentinel. Empty strings are
// still filtered (kept as a no-op for consistency with the create path).
// Returns nil iff the argument is missing/malformed or contains only empty strings.
func extractPropertiesAllowNil(request mcp.CallToolRequest) map[string]interface{} {
	props, ok := parsePropertiesArg(request)
	if !ok {
		return nil
	}
	return filterProperties(props, true)
}

// parsePropertiesArg extracts the raw properties map from a tool request, handling
// both the native map argument and the JSON-encoded string fallback. Returns
// (nil, false) when the argument is missing, of an unsupported type, or malformed/null JSON.
func parsePropertiesArg(request mcp.CallToolRequest) (map[string]interface{}, bool) {
	args := request.GetArguments()
	propsRaw, ok := args["properties"]
	if !ok {
		return nil, false
	}

	switch p := propsRaw.(type) {
	case map[string]interface{}:
		return p, true
	case string:
		var props map[string]interface{}
		if err := json.Unmarshal([]byte(p), &props); err != nil {
			return nil, false
		}
		// JSON `null` unmarshals into a nil map; treat as malformed/missing.
		if props == nil {
			return nil, false
		}
		return props, true
	default:
		return nil, false
	}
}

// filterProperties removes empty-string values from a property map. When
// keepNil is false, nil values are also removed. Returns nil if the resulting
// map is empty.
func filterProperties(props map[string]interface{}, keepNil bool) map[string]interface{} {
	if props == nil {
		return nil
	}
	filtered := make(map[string]interface{}, len(props))
	for k, v := range props {
		if !keepNil && v == nil {
			continue
		}
		if s, ok := v.(string); ok && s == "" {
			continue
		}
		filtered[k] = v
	}
	if len(filtered) == 0 {
		return nil
	}
	return filtered
}

// validatePropertyNames checks property names against the metamodel for the given entity type.
// Reports unknown property names and rejects nil values targeting required properties
// (a nil value means "delete" in update_entity; deleting a required property would leave
// the entity invalid, so we surface that as an actionable error rather than a misleading
// success that analyze_validations later catches).
func (s *Server) validatePropertyNames(entityType string, properties map[string]interface{}) *mcp.CallToolResult {
	if properties == nil {
		return nil
	}

	meta := s.deps.Meta
	entityDef, ok := meta.GetEntityDef(entityType)
	if !ok {
		return nil // Type validation will catch this
	}

	var unknown, requiredDeletes []string
	for propName, v := range properties {
		def, exists := entityDef.Properties[propName]
		if !exists {
			unknown = append(unknown, propName)
			continue
		}
		if v == nil && def.Required {
			requiredDeletes = append(requiredDeletes, propName)
		}
	}

	if len(unknown) > 0 {
		valid := make([]string, 0, len(entityDef.Properties))
		for name := range entityDef.Properties {
			valid = append(valid, name)
		}
		return mcp.NewToolResultError(fmt.Sprintf(
			"unknown properties for %s: %s (valid: %s)",
			entityType, strings.Join(unknown, ", "), strings.Join(valid, ", ")))
	}

	if len(requiredDeletes) > 0 {
		return mcp.NewToolResultError(fmt.Sprintf(
			"cannot delete required properties for %s: %s (set a new value instead)",
			entityType, strings.Join(requiredDeletes, ", ")))
	}

	return nil
}

func applyPagination[T any](items []T, offset, limit int) []T {
	if offset > 0 {
		if offset >= len(items) {
			return nil
		}
		items = items[offset:]
	}
	if limit > 0 && limit < len(items) {
		items = items[:limit]
	}
	return items
}
