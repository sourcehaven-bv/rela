// coverage-ignore: MCP tool handlers - tested via integration tests
package mcp

import (
	"context"
	"fmt"

	mcpgo "github.com/modelcontextprotocol/go-sdk/mcp"
	"gopkg.in/yaml.v3"

	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/schema"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

func (s *Server) handleAnalyzeOrphans(
	ctx context.Context, request *mcpgo.CallToolRequest,
) (*mcpgo.CallToolResult, error) {
	args := newToolRequest(request)
	entityType := args.GetString("type", "")

	orphanIDs, _ := s.deps.Tracer.FindOrphans(ctx)

	st := s.deps.Store
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
		return textResult("No orphan entities found"), nil
	}

	text, err := marshalJSON(orphans)
	if err != nil {
		return errorResult(err.Error()), nil
	}
	return textResult(
		fmt.Sprintf("Found %d orphan entities:\n\n%s", len(orphans), text)), nil
}

type cardinalityViolation struct {
	EntityID string `json:"entity_id"`
	Relation string `json:"relation"`
	Message  string `json:"message"`
}

func (s *Server) handleAnalyzeCardinality(
	ctx context.Context, _ *mcpgo.CallToolRequest,
) (*mcpgo.CallToolResult, error) {
	violations := make([]cardinalityViolation, 0) //nolint:prealloc // capacity unknown

	for relName, relDef := range s.deps.Meta.Relations {
		violations = append(violations, s.checkCardinalityForRelation(ctx, relName, relDef)...)
	}

	if len(violations) == 0 {
		return textResult("All cardinality constraints satisfied"), nil
	}

	text, err := marshalJSON(violations)
	if err != nil {
		return errorResult(err.Error()), nil
	}
	return textResult(
		fmt.Sprintf("Found %d cardinality violations:\n\n%s", len(violations), text)), nil
}

func (s *Server) checkCardinalityForRelation(
	ctx context.Context, relName string, relDef metamodel.RelationDef,
) []cardinalityViolation {
	violations := make([]cardinalityViolation, 0) //nolint:prealloc // capacity unknown

	violations = append(violations,
		s.checkCardinalityBound(ctx, relName, relDef.From, relDef.MinOutgoing, relDef.MaxOutgoing, true)...)

	violations = append(violations,
		s.checkCardinalityBound(ctx, relName, relDef.To, relDef.MinIncoming, relDef.MaxIncoming, false)...)

	return violations
}

func (s *Server) checkCardinalityBound(
	ctx context.Context, relName string, entityTypes []string, minVal, maxVal *int, outgoing bool,
) []cardinalityViolation {
	var violations []cardinalityViolation

	st := s.deps.Store
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

type uniqueViolation struct {
	EntityType string   `json:"entity_type"`
	Property   string   `json:"property"`
	Value      string   `json:"value"`
	EntityIDs  []string `json:"entity_ids"`
}

// handleAnalyzeUnique reports same-type entities that share a value for a
// property declared `unique: true` — collisions the write path rejects on
// new writes but which may already exist in older data. Mirrors the
// read-side analysis.Service.FindUniqueViolations; kept here against
// Store+Meta because the MCP server has no analysis.Service dependency.
func (s *Server) handleAnalyzeUnique(
	ctx context.Context, _ *mcpgo.CallToolRequest,
) (*mcpgo.CallToolResult, error) {
	violations := make([]uniqueViolation, 0)

	for typeName, def := range s.deps.Meta.Entities {
		for propName, pd := range def.PropertyDefs() {
			if !pd.Unique || pd.List {
				continue
			}
			byValue := map[string][]string{}
			for e, err := range s.deps.Store.ListEntities(ctx, store.EntityQuery{Type: typeName}) {
				if err != nil {
					break
				}
				if v := e.GetString(propName); v != "" {
					byValue[v] = append(byValue[v], e.ID)
				}
			}
			for value, ids := range byValue {
				if len(ids) > 1 {
					violations = append(violations, uniqueViolation{
						EntityType: typeName, Property: propName, Value: value, EntityIDs: ids,
					})
				}
			}
		}
	}

	if len(violations) == 0 {
		return textResult("No unique constraint violations found"), nil
	}
	text, err := marshalJSON(violations)
	if err != nil {
		return errorResult(err.Error()), nil
	}
	return textResult(
		fmt.Sprintf("Found %d unique constraint violations:\n\n%s", len(violations), text)), nil
}

func (s *Server) handleAnalyzeProperties(
	ctx context.Context, _ *mcpgo.CallToolRequest,
) (*mcpgo.CallToolResult, error) {
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

	meta := s.deps.Meta
	st := s.deps.Store
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
	relErrors := schema.ValidateRelationProperties(ctx, s.deps.Store, s.deps.Meta)
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
		return textResult("All entity and relation properties are valid"), nil
	}

	result := make(map[string]any)
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
		return errorResult(err.Error()), nil
	}

	return textResult(
		fmt.Sprintf("Found %d property errors across %d entities and %d relations:\n\n%s",
			errorCount, totalEntityErrors, totalRelationErrors, text)), nil
}

func (s *Server) handleAnalyzeValidations(
	ctx context.Context, _ *mcpgo.CallToolRequest,
) (*mcpgo.CallToolResult, error) {
	rules := s.deps.Meta.Validations
	if len(rules) == 0 {
		return textResult("No custom validation rules defined in metamodel"), nil
	}

	type ruleResult struct {
		Rule       string   `json:"rule"`
		Severity   string   `json:"severity"`
		Violations []string `json:"violations"`
	}

	validator := s.deps.Validator
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
		return textResult(
			fmt.Sprintf("All %d validation rules passed", len(rules))), nil
	}

	text, err := marshalJSON(results)
	if err != nil {
		return errorResult(err.Error()), nil
	}
	return textResult(
		"Found validation issues:\n\n" + text), nil
}

func (s *Server) handleAnalyzeSchema(
	ctx context.Context, request *mcpgo.CallToolRequest,
) (*mcpgo.CallToolResult, error) {
	args := newToolRequest(request)
	threshold := args.GetInt("threshold", 0)

	dataEntry := s.loadDataEntryConfig(ctx)

	counter := schema.NewStoreCounter(ctx, s.deps.Store)
	analysis := schema.Analyze(s.deps.Meta, counter, dataEntry, threshold)

	if !analysis.HasIssues() {
		return textResult("All schema types are in use"), nil
	}

	text, err := marshalJSON(analysis)
	if err != nil {
		return errorResult(err.Error()), nil
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

	return textResult(message), nil
}

// loadDataEntryConfig loads data-entry.yaml if it exists.
func (s *Server) loadDataEntryConfig(ctx context.Context) *dataentryconfig.Config {
	data, err := s.deps.Config.Load(ctx, dataentryconfig.ConfigFile)
	if err != nil {
		return nil
	}
	var cfg dataentryconfig.Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil
	}
	return &cfg
}
