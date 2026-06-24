package dataentry

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/lua"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/natsort"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/tracer"
	"github.com/Sourcehaven-BV/rela/internal/validator"
)

// analyzeService runs the read-only graph-analysis checks (orphans,
// duplicates, gaps, cardinality, properties, validations). Extracted from App
// (TKT-N26KLB M5.1): it depends only on the stable read services, and each
// method takes the per-request metamodel snapshot explicitly (capture-once,
// per the project's snapshot rule) rather than reaching back into App.
//
// The whole-graph scans are intentionally ungated; visibility is applied to
// the resulting issues at the HTTP boundary (see visibleAnalysisIssues).
type analyzeService struct {
	store     store.Store
	tracer    tracer.Tracer
	validator validator.Validator
}

// AnalysisIssue represents a single validation issue, optionally linked to an entity.
type AnalysisIssue struct {
	EntityID   string // Empty for non-entity issues (e.g., ID gaps)
	EntityType string
	Title      string
	Message    string
	Severity   string // "error" or "warning"

	// ScriptError carries the raw *lua.ScriptError for validation
	// rules whose Lua script failed. Non-nil only on script-error
	// rows; the HTTP handler converts it to a wire envelope using
	// the per-request loopback gate, so the structured detail
	// (path, source slice, stack) reaches the frontend's existing
	// ScriptErrorDialog rather than a flat string.
	// LoadErrors do NOT get a ScriptError — they're not Lua failures.
	ScriptError *lua.ScriptError
}

// AnalysisSection groups issues by analysis category.
type AnalysisSection struct {
	Name        string
	Description string
	Issues      []AnalysisIssue
}

// ErrorCount returns the number of error-severity issues in this section.
func (s AnalysisSection) ErrorCount() int {
	n := 0
	for _, issue := range s.Issues {
		if issue.Severity == "error" {
			n++
		}
	}
	return n
}

// WarningCount returns the number of warning-severity issues in this section.
func (s AnalysisSection) WarningCount() int {
	n := 0
	for _, issue := range s.Issues {
		if issue.Severity == "warning" {
			n++
		}
	}
	return n
}

// AnalysisResult is the complete output of running all analyses.
type AnalysisResult struct {
	Sections     []AnalysisSection
	ErrorCount   int
	WarningCount int
}

// runAnalysis executes all analysis checks and returns a combined result.
func (svc analyzeService) runAnalysis(ctx context.Context, meta *metamodel.Metamodel) AnalysisResult {
	sections := []AnalysisSection{
		svc.analyzeProperties(ctx, meta),
		svc.analyzeCardinality(ctx, meta),
		svc.analyzeValidations(ctx, meta),
		svc.analyzeOrphans(ctx, meta),
		svc.analyzeDuplicates(ctx, meta),
		svc.analyzeGaps(ctx, meta),
	}

	var errors, warnings int
	for _, s := range sections {
		errors += s.ErrorCount()
		warnings += s.WarningCount()
	}

	return AnalysisResult{
		Sections:     sections,
		ErrorCount:   errors,
		WarningCount: warnings,
	}
}

// analysisIssueCounts returns just the total error and warning counts
// without building the full issue details. Used by the dashboard.
func (svc analyzeService) analysisIssueCounts(ctx context.Context, meta *metamodel.Metamodel) (errors, warnings int) {
	result := svc.runAnalysis(ctx, meta)
	return result.ErrorCount, result.WarningCount
}

// analyzeOrphans finds entities with no connections.
func (svc analyzeService) analyzeOrphans(ctx context.Context, meta *metamodel.Metamodel) AnalysisSection {
	section := AnalysisSection{
		Name:        "Orphans",
		Description: "Entities with no incoming or outgoing relations",
	}

	orphanIDs, _ := svc.tracer.FindOrphans(ctx)

	var orphans []*entity.Entity
	st := svc.store
	for _, id := range orphanIDs {
		if e, err := st.GetEntity(ctx, id); err == nil {
			orphans = append(orphans, e)
		}
	}
	sortStoreEntitiesByID(orphans)

	for _, e := range orphans {
		section.Issues = append(section.Issues, AnalysisIssue{
			EntityID:   e.ID,
			EntityType: e.Type,
			Title:      meta.DisplayTitle(e.ID, e.Type, e.Properties),
			Message:    "No relations",
			Severity:   "warning",
		})
	}

	return section
}

// analyzeDuplicates finds entities with identical normalized titles.
func (svc analyzeService) analyzeDuplicates(ctx context.Context, meta *metamodel.Metamodel) AnalysisSection {
	section := AnalysisSection{
		Name:        "Duplicates",
		Description: "Entities with identical titles",
	}

	titleGroups := make(map[string][]*entity.Entity)
	for e, err := range svc.store.ListEntities(ctx, store.EntityQuery{}) {
		if err != nil {
			break
		}
		title := normalizeTitle(meta.DisplayTitle(e.ID, e.Type, e.Properties))
		if title != "" {
			titleGroups[title] = append(titleGroups[title], e)
		}
	}

	// Collect groups with duplicates, sorted by title
	var titles []string
	for title, group := range titleGroups {
		if len(group) > 1 {
			titles = append(titles, title)
		}
	}
	natsort.Strings(titles)

	for _, title := range titles {
		group := titleGroups[title]
		sortStoreEntitiesByID(group)
		ids := make([]string, len(group))
		for i, e := range group {
			ids[i] = e.ID
		}
		for _, e := range group {
			section.Issues = append(section.Issues, AnalysisIssue{
				EntityID:   e.ID,
				EntityType: e.Type,
				Title:      meta.DisplayTitle(e.ID, e.Type, e.Properties),
				Message:    fmt.Sprintf("Duplicate title (shared by %s)", strings.Join(ids, ", ")),
				Severity:   "warning",
			})
		}
	}

	return section
}

// analyzeGaps finds gaps in ID sequences for auto-numbered entity types.
func (svc analyzeService) analyzeGaps(ctx context.Context, meta *metamodel.Metamodel) AnalysisSection {
	section := AnalysisSection{
		Name:        "ID Gaps",
		Description: "Missing numbers in auto-generated ID sequences",
	}

	// Build prefix → entity type lookup and the manual-prefix skip set
	// in a single pass over the metamodel.
	manualPrefixes := make(map[string]bool)
	typeByPrefix := make(map[string]string)
	for typeName, entityDef := range meta.Entities {
		for _, idPrefix := range entityDef.GetIDPrefixes() {
			trimmed := strings.TrimSuffix(idPrefix, "-")
			if entityDef.IsManualID() {
				manualPrefixes[trimmed] = true
				continue
			}
			typeByPrefix[trimmed] = typeName
		}
	}

	// Group IDs by prefix
	prefixGroups := make(map[string][]int)
	for e, err := range svc.store.ListEntities(ctx, store.EntityQuery{}) {
		if err != nil {
			break
		}
		parsed, err := entity.ParseEntityID(e.ID)
		if err != nil || parsed.Prefix == "" {
			continue
		}
		if manualPrefixes[strings.TrimSuffix(parsed.Prefix, "-")] {
			continue
		}
		prefixGroups[parsed.Prefix] = append(prefixGroups[parsed.Prefix], parsed.Number)
	}

	// Sort prefixes for deterministic output
	prefixes := make([]string, 0, len(prefixGroups))
	for prefix := range prefixGroups {
		prefixes = append(prefixes, prefix)
	}
	natsort.Strings(prefixes)

	for _, prefix := range prefixes {
		numbers := prefixGroups[prefix]
		sort.Ints(numbers)

		var gaps []int
		for i := 1; i < len(numbers); i++ {
			for j := numbers[i-1] + 1; j < numbers[i]; j++ {
				gaps = append(gaps, j)
			}
		}

		// EntityType is populated from the prefix → type map so the
		// data-entry UI's type column renders the type badge. The row
		// stays inert (EntityID is empty), so isClickable in the SPA
		// remains false; the type is informational only.
		entityType := typeByPrefix[strings.TrimSuffix(prefix, "-")]
		for _, n := range gaps {
			missingID := fmt.Sprintf("%s%03d", prefix, n)
			section.Issues = append(section.Issues, AnalysisIssue{
				EntityType: entityType,
				Message:    "Missing ID: " + missingID,
				Severity:   "warning",
			})
		}
	}

	return section
}

// analyzeCardinality checks relation cardinality constraints.
func (svc analyzeService) analyzeCardinality(ctx context.Context, meta *metamodel.Metamodel) AnalysisSection {
	section := AnalysisSection{
		Name:        "Cardinality",
		Description: "Relation cardinality constraint violations",
	}

	st := svc.store

	// Sort relation names for deterministic output
	relNames := make([]string, 0, len(meta.Relations))
	for name := range meta.Relations {
		relNames = append(relNames, name)
	}
	natsort.Strings(relNames)

	// listEntities lists entities of a given type, sorted by ID.
	listEntities := func(t string) []*entity.Entity {
		var out []*entity.Entity
		for e, err := range st.ListEntities(ctx, store.EntityQuery{Type: t}) {
			if err != nil {
				break
			}
			out = append(out, e)
		}
		sortStoreEntitiesByID(out)
		return out
	}

	// countRelations counts relations of a specific type for an entity.
	countRelations := func(entityID, relType string, direction store.Direction) int {
		n, _ := st.CountRelations(ctx, store.RelationQuery{
			EntityID: entityID, Type: relType, Direction: direction,
		})
		return n
	}

	for _, relName := range relNames {
		relDef := meta.Relations[relName]

		// Check min_outgoing
		if relDef.MinOutgoing != nil && *relDef.MinOutgoing > 0 {
			for _, sourceType := range relDef.From {
				for _, e := range listEntities(sourceType) {
					count := countRelations(e.ID, relName, store.DirectionOutgoing)
					if count < *relDef.MinOutgoing {
						section.Issues = append(section.Issues, AnalysisIssue{
							EntityID:   e.ID,
							EntityType: e.Type,
							Title:      meta.DisplayTitle(e.ID, e.Type, e.Properties),
							Message:    fmt.Sprintf("Must have at least %d '%s' relation(s), has %d", *relDef.MinOutgoing, relName, count),
							Severity:   "error",
						})
					}
				}
			}
		}

		// Check max_outgoing
		if relDef.MaxOutgoing != nil {
			for _, sourceType := range relDef.From {
				for _, e := range listEntities(sourceType) {
					count := countRelations(e.ID, relName, store.DirectionOutgoing)
					if count > *relDef.MaxOutgoing {
						section.Issues = append(section.Issues, AnalysisIssue{
							EntityID:   e.ID,
							EntityType: e.Type,
							Title:      meta.DisplayTitle(e.ID, e.Type, e.Properties),
							Message:    fmt.Sprintf("Has more than %d '%s' relation(s): %d", *relDef.MaxOutgoing, relName, count),
							Severity:   "error",
						})
					}
				}
			}
		}

		// Check min_incoming
		if relDef.MinIncoming != nil && *relDef.MinIncoming > 0 {
			for _, targetType := range relDef.To {
				for _, e := range listEntities(targetType) {
					count := countRelations(e.ID, relName, store.DirectionIncoming)
					if count < *relDef.MinIncoming {
						relLabel := relName
						if relDef.Inverse != nil && relDef.Inverse.GetID() != "" {
							relLabel = relDef.Inverse.GetID()
						}
						section.Issues = append(section.Issues, AnalysisIssue{
							EntityID:   e.ID,
							EntityType: e.Type,
							Title:      meta.DisplayTitle(e.ID, e.Type, e.Properties),
							Message:    fmt.Sprintf("Must have at least %d '%s' relation(s), has %d", *relDef.MinIncoming, relLabel, count),
							Severity:   "error",
						})
					}
				}
			}
		}

		// Check max_incoming
		if relDef.MaxIncoming != nil {
			for _, targetType := range relDef.To {
				for _, e := range listEntities(targetType) {
					count := countRelations(e.ID, relName, store.DirectionIncoming)
					if count > *relDef.MaxIncoming {
						relLabel := relName
						if relDef.Inverse != nil && relDef.Inverse.GetID() != "" {
							relLabel = relDef.Inverse.GetID()
						}
						section.Issues = append(section.Issues, AnalysisIssue{
							EntityID:   e.ID,
							EntityType: e.Type,
							Title:      meta.DisplayTitle(e.ID, e.Type, e.Properties),
							Message:    fmt.Sprintf("Has more than %d '%s' relation(s): %d", *relDef.MaxIncoming, relLabel, count),
							Severity:   "error",
						})
					}
				}
			}
		}
	}

	return section
}

// analyzeProperties validates all entity properties against the metamodel.
func (svc analyzeService) analyzeProperties(ctx context.Context, meta *metamodel.Metamodel) AnalysisSection {
	section := AnalysisSection{
		Name:        "Properties",
		Description: "Property validation errors (required fields, invalid values, ID patterns)",
	}

	entities := make([]*entity.Entity, 0)
	for e, err := range svc.store.ListEntities(ctx, store.EntityQuery{}) {
		if err != nil {
			break
		}
		entities = append(entities, e)
	}
	sortStoreEntitiesByID(entities)

	for _, e := range entities {
		errs := meta.ValidateEntity(e.ID, e.Type, e.Properties)
		for _, err := range errs {
			section.Issues = append(section.Issues, AnalysisIssue{
				EntityID:   e.ID,
				EntityType: e.Type,
				Title:      meta.DisplayTitle(e.ID, e.Type, e.Properties),
				Message:    err.Error(),
				Severity:   "error",
			})
		}
	}

	return section
}

// analyzeValidations runs custom validation rules from the metamodel.
//
// The browser surface uses CheckRuleFull so Lua-script failures
// (compile, runtime, timeout, contract) and load failures
// (lua_file: missing, traversal-rejected) appear as error issues
// alongside per-entity violations. Without this, broken Lua rules
// would vanish silently from the data-entry analyze view.
func (svc analyzeService) analyzeValidations(ctx context.Context, meta *metamodel.Metamodel) AnalysisSection {
	section := AnalysisSection{
		Name:        "Validations",
		Description: "Custom validation rules defined in the metamodel",
	}

	st := svc.store
	validator := svc.validator

	for _, rule := range meta.Validations {
		full, err := validator.CheckRuleFull(ctx, rule)
		if err != nil {
			continue
		}
		severity := rule.GetSeverity()
		for _, id := range full.Violations {
			e, err := st.GetEntity(ctx, id)
			if err != nil {
				continue
			}
			section.Issues = append(section.Issues, AnalysisIssue{
				EntityID:   e.ID,
				EntityType: e.Type,
				Title:      meta.DisplayTitle(e.ID, e.Type, e.Properties),
				Message:    rule.Description,
				Severity:   severity,
			})
		}
		// Surface Lua failures and load failures so the UI shows
		// "rule did not run" rather than silently dropping them.
		// These are always error severity — a broken rule is not a
		// warning condition, it's a config-level problem the
		// operator needs to see.
		for _, se := range full.ScriptErrors {
			section.Issues = append(section.Issues, AnalysisIssue{
				EntityID:    se.EntityID,
				EntityType:  "",
				Title:       rule.Name,
				Message:     "Validation script failed: " + scriptErrorSummary(se),
				Severity:    "error",
				ScriptError: se,
			})
		}
		for _, le := range full.LoadErrors {
			section.Issues = append(section.Issues, AnalysisIssue{
				Title:    le.RuleName,
				Message:  "Validation script load failed: " + le.Message,
				Severity: "error",
			})
		}
	}

	return section
}

// scriptErrorSummary builds a single-line summary for the AnalysisIssue
// Message field. The full structured envelope (path, line, source slice)
// is kept on the lua.ScriptError; the browser surface today displays only
// flat strings, so we collapse to a one-liner.
func scriptErrorSummary(se *lua.ScriptError) string {
	if se == nil {
		return ""
	}
	msg := se.Error()
	// Replace newlines so a multi-line wrapped error renders as a
	// single AnalysisIssue.Message rather than corrupting the JSON
	// shape consumers expect.
	msg = strings.ReplaceAll(msg, "\n", " ")
	msg = strings.ReplaceAll(msg, "\r", " ")
	return strings.Join(strings.Fields(msg), " ")
}

func sortStoreEntitiesByID(entities []*entity.Entity) {
	sort.Slice(entities, func(i, j int) bool {
		return natsort.Less(entities[i].ID, entities[j].ID)
	})
}

// normalizeTitle normalizes a title for duplicate comparison.
func normalizeTitle(s string) string {
	s = strings.ToLower(s)
	s = strings.TrimSpace(s)
	fields := strings.Fields(s)
	return strings.Join(fields, " ")
}
