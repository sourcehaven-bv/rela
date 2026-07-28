package dataentry

import (
	"context"
	"fmt"
	"iter"
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

// analyzeReader is the narrow, consumer-side ENTITY-read surface the analyze
// checks need. It is satisfied structurally by store.Store AND by the ctx-gating
// visibility.ScriptReader / Unrestricted readers. Wiring a GATED reader here
// (per TKT-3FL2S6, superseding DEC-O59WM4) makes the whole-graph scans read only
// the requester's slice: a hidden entity produces no issue, and a visible
// entity comes back REDACTED, so its value cannot reach an issue title or a
// validation message. The leak closes by construction, not by filtering output.
//
// Only entity reads: analyze never mutates. Relation COUNTS (cardinality) go
// through a separate counter that stays on the raw store — a count is a
// structural fact, not a value, so it cannot leak; under-visibility only makes
// cardinality potentially false-positive, which is correctness (guarded by the
// roles annotation, arc step 2), not a disclosure.
type analyzeReader interface {
	GetEntity(ctx context.Context, id string) (*entity.Entity, error)
	ListEntities(ctx context.Context, q store.EntityQuery) iter.Seq2[*entity.Entity, error]
}

// relationCounter counts relations for the cardinality check. Raw (ungated) on
// purpose — see analyzeReader.
type relationCounter interface {
	CountRelations(ctx context.Context, q store.RelationQuery) (int, error)
}

// analyzeService runs the read-only graph-analysis checks (orphans,
// duplicates, gaps, cardinality, properties, validations). Extracted from App
// (TKT-N26KLB M5.1): it depends only on the stable read services, and each
// method takes the per-request metamodel snapshot explicitly (capture-once,
// per the project's snapshot rule) rather than reaching back into App.
//
// reads is GATED per the requesting principal (TKT-3FL2S6, DEC-O59WM4
// superseded); relCounts is the raw relation counter (structural, cannot leak);
// the tracer is the gated decorator. Under NopACL, reads/tracer are the raw
// store/tracer (no gating).
type analyzeService struct {
	reads     analyzeReader
	relCounts relationCounter
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

	// Detail carries optional structured specifics about why the issue
	// fired, beyond the flat Message. For content required-headers
	// violations it holds the missing exact headers. Nil for issues
	// with no structured detail.
	Detail []string

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

	// Each orphan id is re-loaded through the GATED reader before it can become
	// an issue: a hidden entity's GetEntity returns not-found and is dropped, and
	// a visible one is redacted. So even if the tracer yielded a raw id (it does
	// not — svc.tracer is gated too), no hidden entity reaches the wire. Do NOT
	// emit an issue straight from an orphan id/type without this gated re-load —
	// that would reopen the leak this arc closed (TKT-3FL2S6).
	var orphans []*entity.Entity
	st := svc.reads
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
	for e, err := range svc.reads.ListEntities(ctx, store.EntityQuery{}) {
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
	for e, err := range svc.reads.ListEntities(ctx, store.EntityQuery{}) {
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
//
//nolint:gocognit,funlen // cardinality analysis enumerates min/max bounds across every relation def and direction; the branches are the distinct violation cases, not extractable shared logic.
func (svc analyzeService) analyzeCardinality(ctx context.Context, meta *metamodel.Metamodel) AnalysisSection {
	section := AnalysisSection{
		Name:        "Cardinality",
		Description: "Relation cardinality constraint violations",
	}

	// Sort relation names for deterministic output
	relNames := make([]string, 0, len(meta.Relations))
	for name := range meta.Relations {
		relNames = append(relNames, name)
	}
	natsort.Strings(relNames)

	// listEntities lists entities of a given type, sorted by ID. GATED: only the
	// requester's visible entities are considered, so a hidden entity's title
	// cannot reach a cardinality issue.
	listEntities := func(t string) []*entity.Entity {
		var out []*entity.Entity
		for e, err := range svc.reads.ListEntities(ctx, store.EntityQuery{Type: t}) {
			if err != nil {
				break
			}
			out = append(out, e)
		}
		sortStoreEntitiesByID(out)
		return out
	}

	// countRelations counts relations of a specific type for an entity. RAW
	// (ungated): a count is a structural fact, not a value — it cannot leak.
	// Under partial visibility this may over/under-count and produce a false
	// cardinality violation (guarded by the roles annotation, arc step 2).
	countRelations := func(entityID, relType string, direction store.Direction) int {
		n, _ := svc.relCounts.CountRelations(ctx, store.RelationQuery{
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
	for e, err := range svc.reads.ListEntities(ctx, store.EntityQuery{}) {
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

	st := svc.reads
	validator := svc.validator

	for _, rule := range meta.Validations {
		full, err := validator.CheckRuleFull(ctx, rule)
		if err != nil {
			continue
		}
		severity := rule.GetSeverity()
		for _, v := range full.Violations {
			e, err := st.GetEntity(ctx, v.EntityID)
			if err != nil {
				continue
			}
			section.Issues = append(section.Issues, AnalysisIssue{
				EntityID:   e.ID,
				EntityType: e.Type,
				Title:      meta.DisplayTitle(e.ID, e.Type, e.Properties),
				Message:    rule.Description,
				Severity:   severity,
				Detail:     v.Detail,
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
