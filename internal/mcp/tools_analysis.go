// coverage-ignore: MCP tool handlers - tested via integration tests
package mcp

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"gopkg.in/yaml.v3"

	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/schema"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

func (s *Server) handleAnalyzeOrphans(
	ctx context.Context, request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	entityType := request.GetString("type", "")

	orphanIDs, _ := s.ws.Tracer().FindOrphans(ctx)

	st := s.ws.Store()
	resolved := ""
	if entityType != "" {
		resolved = s.resolveType(entityType)
	}

	type orphanInfo struct {
		ID     string `json:"id"`
		Type   string `json:"type"`
		Title  string `json:"title,omitempty"`
		Status string `json:"status,omitempty"`
	}
	orphans := make([]orphanInfo, 0)
	for _, id := range orphanIDs {
		e, err := st.GetEntity(ctx, id)
		if err != nil {
			continue
		}
		if resolved != "" && e.Type != resolved {
			continue
		}
		orphans = append(orphans, orphanInfo{
			ID: e.ID, Type: e.Type, Title: e.Title(), Status: e.Status(),
		})
	}

	if len(orphans) == 0 {
		return mcp.NewToolResultText("No orphan entities found"), nil
	}

	text, err := marshalJSON(orphans)
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

	for relName, relDef := range s.ws.Meta().Relations {
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

	violations = append(violations,
		s.checkCardinalityBound(relName, relDef.From, relDef.MinOutgoing, relDef.MaxOutgoing, true)...)

	violations = append(violations,
		s.checkCardinalityBound(relName, relDef.To, relDef.MinIncoming, relDef.MaxIncoming, false)...)

	return violations
}

func (s *Server) checkCardinalityBound(
	relName string, entityTypes []string, minVal, maxVal *int, outgoing bool,
) []cardinalityViolation {
	var violations []cardinalityViolation

	ctx := context.Background()
	st := s.ws.Store()
	for _, entityType := range entityTypes {
		for e, err := range st.ListEntities(ctx, store.EntityQuery{Type: entityType}) {
			if err != nil {
				break
			}

			dir := store.DirectionOutgoing
			if !outgoing {
				dir = store.DirectionIncoming
			}
			count, _ := st.CountRelations(ctx, store.RelationQuery{
				EntityID: e.ID, Type: relName, Direction: dir,
			})

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
	ctx context.Context, _ mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	type entityErrors struct {
		EntityID   string   `json:"entity_id"`
		EntityType string   `json:"entity_type"`
		Errors     []string `json:"errors"`
	}

	type relationErrors struct {
		RelationKey  string   `json:"relation_key"`
		RelationType string   `json:"relation_type"`
		Errors       []string `json:"errors"`
	}

	meta := s.ws.Meta()
	st := s.ws.Store()
	var allEntityErrors []entityErrors

	// Validate entity properties
	for e, err := range st.ListEntities(ctx, store.EntityQuery{}) {
		if err != nil {
			break
		}
		errs := meta.ValidateEntity(e.ID, e.Type, e.Properties)
		if len(errs) > 0 {
			errStrings := make([]string, len(errs))
			for i, ve := range errs {
				errStrings[i] = ve.Error()
			}
			allEntityErrors = append(allEntityErrors, entityErrors{
				EntityID:   e.ID,
				EntityType: e.Type,
				Errors:     errStrings,
			})
		}
	}

	// Validate relation properties
	relErrors := schema.ValidateRelationProperties(s.ws.Store(), s.ws.Meta())
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
	ctx context.Context, _ mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	rules := s.ws.Meta().Validations
	if len(rules) == 0 {
		return mcp.NewToolResultText("No custom validation rules defined in metamodel"), nil
	}

	type ruleResult struct {
		Rule       string   `json:"rule"`
		Severity   string   `json:"severity"`
		Violations []string `json:"violations"`
	}

	validator := s.ws.Validator()
	var results []ruleResult
	for _, rule := range rules {
		ids, err := validator.CheckRule(ctx, rule)
		if err != nil {
			continue
		}
		if len(ids) > 0 {
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

	dataEntry := s.loadDataEntryConfig()

	counter := &schema.StoreCounter{Store: s.ws.Store()}
	analysis := schema.Analyze(s.ws.Meta(), counter, dataEntry, threshold)

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
	data, err := s.ws.Config().Load(context.Background(), dataentryconfig.ConfigFile)
	if err != nil {
		return nil
	}
	var cfg dataentryconfig.Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil
	}
	return &cfg
}
