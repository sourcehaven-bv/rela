// coverage-ignore: MCP tool handlers - tested via integration tests
package mcp

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"gopkg.in/yaml.v3"

	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/model"
	"github.com/Sourcehaven-BV/rela/internal/schema"
)

func (s *Server) handleAnalyzeOrphans(
	_ context.Context, request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	entityType := request.GetString("type", "")

	snap := s.ws.Snapshot()
	g := snap.Graph()
	orphans := g.FindOrphans()
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
	snap := s.ws.Snapshot()
	var violations []cardinalityViolation

	for relName, relDef := range snap.Meta().Relations {
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

	// Check outgoing constraints (from-side: outgoing edges from source types)
	violations = append(violations,
		s.checkCardinalityBound(relName, relDef.From, relDef.MinOutgoing, relDef.MaxOutgoing, true)...)

	// Check incoming constraints (to-side: incoming edges to target types)
	violations = append(violations,
		s.checkCardinalityBound(relName, relDef.To, relDef.MinIncoming, relDef.MaxIncoming, false)...)

	return violations
}

func (s *Server) checkCardinalityBound(
	relName string, entityTypes []string, minVal, maxVal *int, outgoing bool,
) []cardinalityViolation {
	var violations []cardinalityViolation

	snap := s.ws.Snapshot()
	g := snap.Graph()
	for _, entityType := range entityTypes {
		for _, e := range g.NodesByType(entityType) {
			var edges []*model.Relation
			if outgoing {
				edges = g.OutgoingEdges(e.ID)
			} else {
				edges = g.IncomingEdges(e.ID)
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

	type relationErrors struct {
		RelationKey  string   `json:"relation_key"` // from--type--to
		RelationType string   `json:"relation_type"`
		Errors       []string `json:"errors"`
	}

	snap := s.ws.Snapshot()
	meta := snap.Meta()
	var allEntityErrors []entityErrors

	// Validate entity properties
	for _, entity := range snap.Graph().AllNodes() {
		errs := meta.ValidateEntity(entity)
		if len(errs) > 0 {
			errStrings := make([]string, len(errs))
			for i, e := range errs {
				errStrings[i] = e.Error()
			}
			allEntityErrors = append(allEntityErrors, entityErrors{
				EntityID:   entity.ID,
				EntityType: entity.Type,
				Errors:     errStrings,
			})
		}
	}

	// Validate relation properties
	relErrors := s.ws.ValidateRelationProperties()
	allRelationErrors := make([]relationErrors, 0, len(relErrors))
	for _, rpe := range relErrors {
		errStrings := make([]string, len(rpe.Errors))
		for i, e := range rpe.Errors {
			errStrings[i] = e.Error()
		}
		allRelationErrors = append(allRelationErrors, relationErrors{
			RelationKey:  rpe.RelationKey,
			RelationType: rpe.RelationType,
			Errors:       errStrings,
		})
	}

	totalEntityErrors := len(allEntityErrors)
	totalRelationErrors := len(allRelationErrors)

	if totalEntityErrors == 0 && totalRelationErrors == 0 {
		return mcp.NewToolResultText("All entity and relation properties are valid"), nil
	}

	// Build combined result
	result := make(map[string]interface{})
	errorCount := 0

	if totalEntityErrors > 0 {
		result["entities"] = allEntityErrors
		for _, ee := range allEntityErrors {
			errorCount += len(ee.Errors)
		}
	}

	if totalRelationErrors > 0 {
		result["relations"] = allRelationErrors
		for _, re := range allRelationErrors {
			errorCount += len(re.Errors)
		}
	}

	text, err := marshalJSON(result)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	return mcp.NewToolResultText(
		fmt.Sprintf("Found %d property errors across %d entities and %d relations:\n\n%s",
			errorCount, totalEntityErrors, totalRelationErrors, text)), nil
}

func (s *Server) handleAnalyzeValidations(
	_ context.Context, _ mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	snap := s.ws.Snapshot()
	rules := snap.Meta().Validations
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

func (s *Server) handleAnalyzeSchema(
	_ context.Context, request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	threshold := request.GetInt("threshold", 0)

	// Load optional config files
	dataEntry := s.loadDataEntryConfig()
	viewsFile, _ := s.loadViews()

	// Run analysis
	snap := s.ws.Snapshot()
	analysis := schema.Analyze(snap.Meta(), snap.Graph(), dataEntry, viewsFile, threshold)

	if !analysis.HasIssues() {
		return mcp.NewToolResultText("All schema types are in use"), nil
	}

	text, err := marshalJSON(analysis)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	totalUnused := analysis.TotalUnused()
	totalLowUsage := analysis.TotalLowUsage()

	var message string
	if totalLowUsage > 0 {
		message = fmt.Sprintf("Found %d unused types and %d low-usage types:\n\n%s",
			totalUnused, totalLowUsage, text)
	} else {
		message = fmt.Sprintf("Found %d unused types:\n\n%s", totalUnused, text)
	}

	return mcp.NewToolResultText(message), nil
}

// loadDataEntryConfig loads data-entry.yaml if it exists.
func (s *Server) loadDataEntryConfig() *dataentryconfig.Config {
	data, err := s.ws.ReadProjectFile(dataentryconfig.ConfigFile)
	if err != nil {
		return nil
	}
	var cfg dataentryconfig.Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil
	}
	return &cfg
}
