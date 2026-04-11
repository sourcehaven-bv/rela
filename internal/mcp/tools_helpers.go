package mcp

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/Sourcehaven-BV/rela/internal/filter"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/model"
	"github.com/Sourcehaven-BV/rela/internal/workspace"
)

func (s *Server) resolveType(typeName string) string {
	typeName = strings.TrimSpace(typeName)
	meta := s.ws.Snapshot().Meta()
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
	def, ok := s.ws.Snapshot().Meta().GetEntityDef(resolved)
	if !ok {
		return "", nil, fmt.Errorf("unknown entity type: %s", typeName)
	}
	return resolved, def, nil
}

func (s *Server) extractProperties(request mcp.CallToolRequest) map[string]interface{} {
	args := request.GetArguments()
	propsRaw, ok := args["properties"]
	if !ok {
		return nil
	}

	var props map[string]interface{}
	switch p := propsRaw.(type) {
	case map[string]interface{}:
		props = p
	case string:
		// Try to parse as JSON
		if err := json.Unmarshal([]byte(p), &props); err != nil {
			return nil
		}
	default:
		return nil
	}

	// Filter out nil and empty string values - they represent "no value"
	return filterNilAndEmpty(props)
}

// filterNilAndEmpty removes nil and empty string values from a property map.
// These values are semantically "no value" and should not be stored.
func filterNilAndEmpty(props map[string]interface{}) map[string]interface{} {
	if props == nil {
		return nil
	}
	filtered := make(map[string]interface{}, len(props))
	for k, v := range props {
		if v == nil {
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

func (s *Server) validateEntity(entity *model.Entity) *mcp.CallToolResult {
	snap := s.ws.Snapshot()
	errs := snap.Meta().ValidateEntity(entity)
	if len(errs) == 0 {
		return nil
	}
	msgs := make([]string, 0, len(errs))
	for _, e := range errs {
		msgs = append(msgs, e.Error())
	}
	return mcp.NewToolResultError(fmt.Sprintf("validation errors:\n  %s", strings.Join(msgs, "\n  ")))
}

// validatePropertyNames checks if all property names exist in the metamodel for the given entity type.
func (s *Server) validatePropertyNames(entityType string, properties map[string]interface{}) *mcp.CallToolResult {
	if properties == nil {
		return nil
	}

	snap := s.ws.Snapshot()
	meta := snap.Meta()
	entityDef, ok := meta.GetEntityDef(entityType)
	if !ok {
		return nil // Type validation will catch this
	}

	var unknown []string
	for propName := range properties {
		if _, exists := entityDef.Properties[propName]; !exists {
			unknown = append(unknown, propName)
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

	return nil
}

func (s *Server) checkValidationRule(rule metamodel.ValidationRule) []*model.Entity {
	snap := s.ws.Snapshot()
	var entities []*model.Entity
	if rule.EntityType != "" {
		entities = snap.EntitiesByType(rule.EntityType)
	} else {
		entities = snap.AllEntities()
	}
	return workspace.CheckValidationRule(snap.Meta(), rule, entities)
}

func countEdgesByType(edges []*model.Relation, relType string) int {
	count := 0
	for _, e := range edges {
		if e.Type == relType {
			count++
		}
	}
	return count
}

func filterEntities(entities []*model.Entity, where string) ([]*model.Entity, error) {
	f, err := filter.Parse(where)
	if err != nil {
		return nil, err
	}
	var filtered []*model.Entity
	for _, e := range entities {
		val, ok := e.Properties[f.Property]
		if !ok {
			if f.Operator == filter.OpNotEqual {
				filtered = append(filtered, e)
			}
			continue
		}
		if filter.MatchValue(val, f) {
			filtered = append(filtered, e)
		}
	}
	return filtered, nil
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
