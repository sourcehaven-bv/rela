// coverage-ignore: MCP tool handlers - tested via integration tests
package mcp

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/model"
)

func (s *Server) handleAnalyzeOrphans(
	_ context.Context, request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	entityType := request.GetString("type", "")

	orphans := s.graph.FindOrphans()
	if entityType != "" {
		resolved := s.resolveType(entityType)
		var filtered []*model.Entity
		for _, o := range orphans {
			if o.Type == resolved {
				filtered = append(filtered, o)
			}
		}
		orphans = filtered
	}

	if len(orphans) == 0 {
		return mcp.NewToolResultText("No orphan entities found"), nil
	}

	sortEntitiesByID(orphans)
	text, err := convertEntitiesList(orphans)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return mcp.NewToolResultText(
		fmt.Sprintf("Found %d orphan entities:\n\n%s", len(orphans), text)), nil
}

type cardinalityViolation struct {
	EntityID string `json:"entity_id"`
	Relation string `json:"relation"`
	Message  string `json:"message"`
}

func (s *Server) handleAnalyzeCardinality(
	_ context.Context, _ mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	var violations []cardinalityViolation

	for relName, relDef := range s.meta.Relations {
		violations = append(violations, s.checkCardinalityForRelation(relName, relDef)...)
	}

	if len(violations) == 0 {
		return mcp.NewToolResultText("All cardinality constraints satisfied"), nil
	}

	text, err := marshalJSON(violations)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return mcp.NewToolResultText(
		fmt.Sprintf("Found %d cardinality violations:\n\n%s", len(violations), text)), nil
}

func (s *Server) checkCardinalityForRelation(
	relName string, relDef metamodel.RelationDef,
) []cardinalityViolation {
	var violations []cardinalityViolation

	// Check source constraints (outgoing edges from source types)
	violations = append(violations,
		s.checkCardinalityBound(relName, relDef.From, relDef.SourceMin, relDef.SourceMax, true)...)

	// Check target constraints (incoming edges to target types)
	violations = append(violations,
		s.checkCardinalityBound(relName, relDef.To, relDef.TargetMin, relDef.TargetMax, false)...)

	return violations
}

func (s *Server) checkCardinalityBound(
	relName string, entityTypes []string, minVal, maxVal *int, outgoing bool,
) []cardinalityViolation {
	var violations []cardinalityViolation

	for _, entityType := range entityTypes {
		for _, e := range s.graph.NodesByType(entityType) {
			var edges []*model.Relation
			if outgoing {
				edges = s.graph.OutgoingEdges(e.ID)
			} else {
				edges = s.graph.IncomingEdges(e.ID)
			}
			count := countEdgesByType(edges, relName)

			direction := ""
			if !outgoing {
				direction = "incoming "
			}

			if minVal != nil && *minVal > 0 && count < *minVal {
				violations = append(violations, cardinalityViolation{
					EntityID: e.ID, Relation: relName,
					Message: fmt.Sprintf("must have at least %d %s'%s' relation(s), has %d",
						*minVal, direction, relName, count),
				})
			}
			if maxVal != nil && count > *maxVal {
				violations = append(violations, cardinalityViolation{
					EntityID: e.ID, Relation: relName,
					Message: fmt.Sprintf("has more than %d %s'%s' relation(s): %d",
						*maxVal, direction, relName, count),
				})
			}
		}
	}

	return violations
}

func (s *Server) handleAnalyzeProperties(
	_ context.Context, _ mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	type entityErrors struct {
		EntityID   string   `json:"entity_id"`
		EntityType string   `json:"entity_type"`
		Errors     []string `json:"errors"`
	}

	var allErrors []entityErrors
	for _, entity := range s.graph.AllNodes() {
		errs := s.meta.ValidateEntity(entity)
		if len(errs) > 0 {
			errStrings := make([]string, len(errs))
			for i, e := range errs {
				errStrings[i] = e.Error()
			}
			allErrors = append(allErrors, entityErrors{
				EntityID:   entity.ID,
				EntityType: entity.Type,
				Errors:     errStrings,
			})
		}
	}

	if len(allErrors) == 0 {
		return mcp.NewToolResultText("All entity properties are valid"), nil
	}

	text, err := marshalJSON(allErrors)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	errorCount := 0
	for _, ee := range allErrors {
		errorCount += len(ee.Errors)
	}
	return mcp.NewToolResultText(
		fmt.Sprintf("Found %d property errors across %d entities:\n\n%s",
			errorCount, len(allErrors), text)), nil
}

func (s *Server) handleAnalyzeValidations(
	_ context.Context, _ mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	rules := s.meta.Validations
	if len(rules) == 0 {
		return mcp.NewToolResultText("No custom validation rules defined in metamodel"), nil
	}

	type ruleResult struct {
		Rule       string   `json:"rule"`
		Severity   string   `json:"severity"`
		Violations []string `json:"violations"`
	}

	var results []ruleResult
	for _, rule := range rules {
		violations := s.checkValidationRule(rule)
		if len(violations) > 0 {
			ids := make([]string, len(violations))
			for i, v := range violations {
				ids[i] = v.ID
			}
			results = append(results, ruleResult{
				Rule:       rule.Description,
				Severity:   rule.GetSeverity(),
				Violations: ids,
			})
		}
	}

	if len(results) == 0 {
		return mcp.NewToolResultText(
			fmt.Sprintf("All %d validation rules passed", len(rules))), nil
	}

	text, err := marshalJSON(results)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return mcp.NewToolResultText(
		fmt.Sprintf("Found validation issues:\n\n%s", text)), nil
}
