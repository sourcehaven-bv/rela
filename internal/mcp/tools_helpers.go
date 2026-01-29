package mcp

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/Sourcehaven-BV/rela/internal/filter"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/model"
)

func (s *Server) resolveType(typeName string) string {
	resolved := s.meta.ResolveAlias(typeName)
	if _, ok := s.meta.GetEntityDef(resolved); ok {
		return resolved
	}
	// Try stripping plural
	for _, suffix := range []string{"ies", "es", "s"} {
		replacements := map[string]string{"ies": "y", "es": "", "s": ""}
		if strings.HasSuffix(typeName, suffix) {
			singular := strings.TrimSuffix(typeName, suffix) + replacements[suffix]
			resolved = s.meta.ResolveAlias(singular)
			if _, ok := s.meta.GetEntityDef(resolved); ok {
				return resolved
			}
		}
	}
	return typeName
}

func (s *Server) resolveEntityType(typeName string) (string, *metamodel.EntityDef, error) {
	resolved := s.resolveType(typeName)
	def, ok := s.meta.GetEntityDef(resolved)
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

	switch p := propsRaw.(type) {
	case map[string]interface{}:
		return p
	case string:
		// Try to parse as JSON
		var result map[string]interface{}
		if err := json.Unmarshal([]byte(p), &result); err == nil {
			return result
		}
	}
	return nil
}

func (s *Server) validateEntity(entity *model.Entity) *mcp.CallToolResult {
	errs := s.meta.ValidateEntity(entity)
	if len(errs) == 0 {
		return nil
	}
	msgs := make([]string, 0, len(errs))
	for _, e := range errs {
		msgs = append(msgs, e.Error())
	}
	return mcp.NewToolResultError(fmt.Sprintf("validation errors:\n  %s", strings.Join(msgs, "\n  ")))
}

func (s *Server) saveCache() {
	if s.projectCtx != nil && s.graph != nil {
		if err := s.graph.SaveCache(s.projectCtx.CachePath); err != nil {
			s.logger.Printf("Warning: failed to save cache: %v", err)
		}
	}
}

func (s *Server) checkValidationRule(rule metamodel.ValidationRule) []*model.Entity {
	whenFilters, err := filter.ParseAll(rule.When)
	if err != nil {
		return nil
	}
	thenFilters, err := filter.ParseAll(rule.Then)
	if err != nil {
		return nil
	}

	var entities []*model.Entity
	if rule.EntityType != "" {
		entities = s.graph.NodesByType(rule.EntityType)
	} else {
		entities = s.graph.AllNodes()
	}

	var violations []*model.Entity
	for _, entity := range entities {
		entityDef, ok := s.meta.GetEntityDef(entity.Type)
		if !ok {
			continue
		}

		if len(whenFilters) > 0 {
			matches, matchErr := filter.MatchAll(entity, whenFilters, entityDef, s.meta)
			if matchErr != nil || !matches {
				continue
			}
		}

		satisfies, matchErr := filter.MatchAll(entity, thenFilters, entityDef, s.meta)
		if matchErr != nil || !satisfies {
			violations = append(violations, entity)
		}
	}

	return violations
}

func matchesSearch(e *model.Entity, queryLower string) bool {
	if strings.Contains(strings.ToLower(e.ID), queryLower) {
		return true
	}
	for _, v := range e.Properties {
		if str, ok := v.(string); ok {
			if strings.Contains(strings.ToLower(str), queryLower) {
				return true
			}
		}
	}
	return strings.Contains(strings.ToLower(e.Content), queryLower)
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
