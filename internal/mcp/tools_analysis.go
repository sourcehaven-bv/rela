// coverage-ignore: MCP tool handlers - tested via integration tests
package mcp

import (
	"context"
	"fmt"

	mcpgo "github.com/modelcontextprotocol/go-sdk/mcp"
	"gopkg.in/yaml.v3"

	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	"github.com/Sourcehaven-BV/rela/internal/schema"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

func (s *Server) handleAnalyzeOrphans(
	ctx context.Context, request *mcpgo.CallToolRequest,
) (*mcpgo.CallToolResult, error) {
	args := newToolRequest(request)
	entityType := args.GetString("type", "")

	orphanIDs, _ := s.deps().Tracer.FindOrphans(ctx)

	st := s.deps().Store
	resolved := ""
	if entityType != "" {
		resolved = group(s, selTypes).resolveType(entityType)
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
	if err != nil { // coverage-ignore: defensive: orphans is []orphanInfo of strings built from store entities;
		// json.Marshal cannot fail.
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

// handleAnalyzeCardinality reports every relation whose declared min/max
// bounds the graph does not satisfy.
//
// The check itself is [schema.CheckCardinality] — the SAME implementation
// `rela analyze cardinality` and `rela validate --check cardinality` run, so
// the MCP surface cannot report a different set of violations than the CLI
// (TKT-CICJSN). It is reached as a free function over Deps.Store rather than
// through an analysis.Service because MCP reads through a gated GraphReader,
// not a store.Store, and must not depend on `internal/analysis`.
//
// A store error fails the TOOL CALL rather than being reported as an empty
// or partial result: a failed count reads as 0, which for a min bound is
// indistinguishable from genuinely missing relations, so reporting around
// one would invent violations out of a backend outage.
//
// Adopting the shared checker changed two things an MCP caller can see, both
// deliberate. An incoming bound is now reported against the relation's
// INVERSE id rather than the forward name prefixed with "incoming ". And the
// subject scan asks for AllStates, so where the reader honors that, a faced
// entity is checked per face (TKT-4Y6CMV) and a content-scoped edge on the
// draft no longer satisfies the published face's bound.
//
// The scan widening is visible on its own, separate from faces: the deleted
// copy scanned the default state only. On a project with stranded or
// undeclared face rows — the condition `rela analyze states` exists to find
// (TKT-DOFYR1, BUG-UA3BK3) — a violation can now name a storage row that no
// ordinary read path surfaces, so an agent may see an id it cannot
// show_entity. That is the detector working, not a leak: the rows are the
// operator's own data and the remedy is a data migration.
//
// Per-face checking is NOT guaranteed for every wiring, and the difference is
// in the reader rather than here. `store.GraphQuery` has no AllStates field,
// so when an ACL-gated reader composes one (visibility.listPushdown's Query
// branch, taken by a principal whose grants compile to a policy query) the
// request is dropped and each id collapses to a single world prime. Such a
// caller checks FEWER subjects than the CLI does over a raw store. That
// direction is safe — an unscanned face means a violation is missed, never
// invented — but do not build on per-face coverage here as if it were
// universal (RR-16R183; closing the gap is a store.GraphQuery change,
// alongside TKT-O7R2A1).
func (s *Server) handleAnalyzeCardinality(
	ctx context.Context, _ *mcpgo.CallToolRequest,
) (*mcpgo.CallToolResult, error) {
	// One snapshot for the whole operation (CLAUDE.md "capture state once"):
	// ReloadDeps republishes the bundle atomically on a schema.yaml edit, so
	// two separate deps() loads could pair a new store with an old metamodel.
	d := s.deps()
	found, err := schema.CheckCardinality(ctx, d.Store, d.Meta, nil)
	if err != nil {
		return errorResult(err.Error()), nil
	}

	violations := make([]cardinalityViolation, 0, len(found))
	for _, v := range found {
		violations = append(violations, cardinalityViolation{
			EntityID: v.EntityID,
			Relation: v.RelationType,
			Message:  v.Message(),
		})
	}

	if len(violations) == 0 {
		return textResult("All cardinality constraints satisfied"), nil
	}

	text, err := marshalJSON(violations)
	if err != nil { // coverage-ignore: defensive: violations is []cardinalityViolation of strings; json.Marshal cannot
		// fail.
		return errorResult(err.Error()), nil
	}
	return textResult(
		fmt.Sprintf("Found %d cardinality violations:\n\n%s", len(violations), text)), nil
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

	for typeName, def := range s.deps().Meta.Entities {
		for propName, pd := range def.PropertyDefs() {
			if !pd.Unique || pd.List {
				continue
			}
			byValue := map[string][]string{}
			for e, err := range s.deps().Store.ListEntities(ctx, store.EntityQuery{Type: typeName}) {
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
	if err != nil { // coverage-ignore: defensive: violations is []uniqueViolation of strings; json.Marshal cannot fail.
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

	meta := s.deps().Meta
	st := s.deps().Store
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
	relErrors := schema.ValidateRelationProperties(ctx, s.deps().Store, s.deps().Meta)
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
	if err != nil { // coverage-ignore: defensive: result is a map of []entityErrors/[]relationErrors (strings);
		// json.Marshal cannot fail.
		return errorResult(err.Error()), nil
	}

	return textResult(
		fmt.Sprintf("Found %d property errors across %d entities and %d relations:\n\n%s",
			errorCount, totalEntityErrors, totalRelationErrors, text)), nil
}

func (s *Server) handleAnalyzeValidations(
	ctx context.Context, _ *mcpgo.CallToolRequest,
) (*mcpgo.CallToolResult, error) {
	rules := s.deps().Meta.Validations
	if len(rules) == 0 {
		return textResult("No custom validation rules defined in metamodel"), nil
	}

	// A violation is an object rather than a bare id so a Lua rule's
	// per-entity message — which of the rule's several possible defects
	// this entity has — reaches the caller. Rule carries the rule's own
	// description, which is the same for every violation of it; the two
	// are different levels and neither substitutes for the other.
	type ruleViolation struct {
		EntityID string `json:"entity_id"`
		// Message is the per-entity explanation returned by a Lua rule.
		// Omitted for non-Lua rules and for a Lua rule that returned no
		// message: the rule description is then the whole finding.
		Message string `json:"message,omitempty"`
	}
	type ruleResult struct {
		Rule       string          `json:"rule"`
		Severity   string          `json:"severity"`
		Violations []ruleViolation `json:"violations"`
	}

	validator := s.deps().Validator
	var results []ruleResult
	for _, rule := range rules {
		full, err := validator.CheckRuleFull(ctx, rule)
		if err != nil {
			continue
		}
		if len(full.Violations) == 0 {
			continue
		}
		violations := make([]ruleViolation, 0, len(full.Violations))
		for _, v := range full.Violations {
			violations = append(violations, ruleViolation{
				EntityID: v.EntityID,
				Message:  v.Message,
			})
		}
		results = append(results, ruleResult{
			Rule:       rule.Description,
			Severity:   rule.GetSeverity(),
			Violations: violations,
		})
	}

	if len(results) == 0 {
		return textResult(
			fmt.Sprintf("All %d validation rules passed", len(rules))), nil
	}

	text, err := marshalJSON(results)
	if err != nil { // coverage-ignore: defensive: results is []ruleResult of strings; json.Marshal cannot fail.
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

	counter := schema.NewStoreCounter(ctx, s.deps().Store)
	analysis := schema.Analyze(s.deps().Meta, counter, dataEntry, threshold)

	if !analysis.HasIssues() {
		return textResult("All schema types are in use"), nil
	}

	text, err := marshalJSON(analysis)
	if err != nil { // coverage-ignore: defensive: analysis is a schema.Analysis of strings/ints; json.Marshal cannot
		// fail.
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
	data, err := s.deps().Config.Load(ctx, dataentryconfig.ConfigFile)
	if err != nil {
		return nil
	}
	var cfg dataentryconfig.Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil
	}
	return &cfg
}
