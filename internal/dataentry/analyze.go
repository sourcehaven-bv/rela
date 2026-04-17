package dataentry

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/model"
	"github.com/Sourcehaven-BV/rela/internal/natsort"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// AnalysisIssue represents a single validation issue, optionally linked to an entity.
type AnalysisIssue struct {
	EntityID   string // Empty for non-entity issues (e.g., ID gaps)
	EntityType string
	Title      string
	Message    string
	Severity   string // "error" or "warning"
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
func (a *App) runAnalysis() AnalysisResult {
	sections := []AnalysisSection{
		a.analyzeProperties(),
		a.analyzeCardinality(),
		a.analyzeValidations(),
		a.analyzeOrphans(),
		a.analyzeDuplicates(),
		a.analyzeGaps(),
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
func (a *App) analysisIssueCounts() (errors, warnings int) {
	result := a.runAnalysis()
	return result.ErrorCount, result.WarningCount
}

// analyzeOrphans finds entities with no connections.
func (a *App) analyzeOrphans() AnalysisSection {
	s := a.State()
	section := AnalysisSection{
		Name:        "Orphans",
		Description: "Entities with no incoming or outgoing relations",
	}

	ctx := context.Background()
	orphanIDs, _ := a.ws.Tracer().FindOrphans(ctx)

	var orphans []*entity.Entity
	st := a.ws.Store()
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
			Title:      s.Meta.DisplayTitle(e.ID, e.Type, e.Properties),
			Message:    "No relations",
			Severity:   "warning",
		})
	}

	return section
}

// analyzeDuplicates finds entities with identical normalized titles.
func (a *App) analyzeDuplicates() AnalysisSection {
	s := a.State()
	section := AnalysisSection{
		Name:        "Duplicates",
		Description: "Entities with identical titles",
	}

	ctx := context.Background()
	titleGroups := make(map[string][]*entity.Entity)
	for e, err := range a.ws.Store().ListEntities(ctx, store.EntityQuery{}) {
		if err != nil {
			break
		}
		title := normalizeTitle(s.Meta.DisplayTitle(e.ID, e.Type, e.Properties))
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
				Title:      s.Meta.DisplayTitle(e.ID, e.Type, e.Properties),
				Message:    fmt.Sprintf("Duplicate title (shared by %s)", strings.Join(ids, ", ")),
				Severity:   "warning",
			})
		}
	}

	return section
}

// analyzeGaps finds gaps in ID sequences for auto-numbered entity types.
func (a *App) analyzeGaps() AnalysisSection {
	s := a.State()
	section := AnalysisSection{
		Name:        "ID Gaps",
		Description: "Missing numbers in auto-generated ID sequences",
	}

	// Build set of manual ID prefixes to skip
	manualPrefixes := make(map[string]bool)
	for _, entityDef := range s.Meta.Entities {
		if entityDef.IsManualID() {
			for _, idPrefix := range entityDef.GetIDPrefixes() {
				manualPrefixes[strings.TrimSuffix(idPrefix, "-")] = true
			}
		}
	}

	// Group IDs by prefix
	ctx := context.Background()
	prefixGroups := make(map[string][]int)
	for e, err := range a.ws.Store().ListEntities(ctx, store.EntityQuery{}) {
		if err != nil {
			break
		}
		parsed, err := model.ParseEntityID(e.ID)
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

		for _, n := range gaps {
			missingID := fmt.Sprintf("%s%03d", prefix, n)
			section.Issues = append(section.Issues, AnalysisIssue{
				Message:  fmt.Sprintf("Missing ID: %s", missingID),
				Severity: "warning",
			})
		}
	}

	return section
}

// analyzeCardinality checks relation cardinality constraints.
func (a *App) analyzeCardinality() AnalysisSection {
	s := a.State()
	section := AnalysisSection{
		Name:        "Cardinality",
		Description: "Relation cardinality constraint violations",
	}

	ctx := context.Background()
	st := a.ws.Store()

	// Sort relation names for deterministic output
	relNames := make([]string, 0, len(s.Meta.Relations))
	for name := range s.Meta.Relations {
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
		relDef := s.Meta.Relations[relName]

		// Check min_outgoing
		if relDef.MinOutgoing != nil && *relDef.MinOutgoing > 0 {
			for _, sourceType := range relDef.From {
				for _, e := range listEntities(sourceType) {
					count := countRelations(e.ID, relName, store.DirectionOutgoing)
					if count < *relDef.MinOutgoing {
						section.Issues = append(section.Issues, AnalysisIssue{
							EntityID:   e.ID,
							EntityType: e.Type,
							Title:      s.Meta.DisplayTitle(e.ID, e.Type, e.Properties),
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
							Title:      s.Meta.DisplayTitle(e.ID, e.Type, e.Properties),
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
							Title:      s.Meta.DisplayTitle(e.ID, e.Type, e.Properties),
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
							Title:      s.Meta.DisplayTitle(e.ID, e.Type, e.Properties),
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
func (a *App) analyzeProperties() AnalysisSection {
	s := a.State()
	section := AnalysisSection{
		Name:        "Properties",
		Description: "Property validation errors (required fields, invalid values, ID patterns)",
	}

	ctx := context.Background()
	var entities []*entity.Entity
	for e, err := range a.ws.Store().ListEntities(ctx, store.EntityQuery{}) {
		if err != nil {
			break
		}
		entities = append(entities, e)
	}
	sortStoreEntitiesByID(entities)

	for _, e := range entities {
		errs := s.Meta.ValidateEntity(e.ID, e.Type, e.Properties)
		for _, err := range errs {
			section.Issues = append(section.Issues, AnalysisIssue{
				EntityID:   e.ID,
				EntityType: e.Type,
				Title:      s.Meta.DisplayTitle(e.ID, e.Type, e.Properties),
				Message:    err.Error(),
				Severity:   "error",
			})
		}
	}

	return section
}

// analyzeValidations runs custom validation rules from the metamodel.
func (a *App) analyzeValidations() AnalysisSection {
	s := a.State()
	section := AnalysisSection{
		Name:        "Validations",
		Description: "Custom validation rules defined in the metamodel",
	}

	ctx := context.Background()
	st := a.ws.Store()
	validator := a.ws.Validator()

	for _, rule := range s.Meta.Validations {
		ids, err := validator.CheckRule(ctx, rule)
		if err != nil {
			continue
		}
		severity := rule.GetSeverity()
		for _, id := range ids {
			e, err := st.GetEntity(ctx, id)
			if err != nil {
				continue
			}
			section.Issues = append(section.Issues, AnalysisIssue{
				EntityID:   e.ID,
				EntityType: e.Type,
				Title:      s.Meta.DisplayTitle(e.ID, e.Type, e.Properties),
				Message:    rule.Description,
				Severity:   severity,
			})
		}
	}

	return section
}

// countEdgesByType counts relations of a specific type in a slice.
func countEdgesByType(edges []*model.Relation, relType string) int {
	n := 0
	for _, e := range edges {
		if e.Type == relType {
			n++
		}
	}
	return n
}

// sortEntitiesByID sorts entities by their ID using natural ordering for deterministic output.
func sortEntitiesByID(entities []*model.Entity) {
	sort.Slice(entities, func(i, j int) bool {
		return natsort.Less(entities[i].ID, entities[j].ID)
	})
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
