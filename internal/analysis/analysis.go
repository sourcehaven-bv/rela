// Package analysis exposes the read-only analysis facades the CLI
// uses to surface graph health: orphans, duplicates, gaps, cardinality
// violations, custom Lua validations, and orphan temp files left by
// interrupted writes. The service depends only on the focused
// primitives it needs (Store, Meta, Tracer, FS, Paths, Lua deps) so it
// can be constructed at any wiring site.
package analysis

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/lua"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/project"
	"github.com/Sourcehaven-BV/rela/internal/schema"
	"github.com/Sourcehaven-BV/rela/internal/storage"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/tracer"
	"github.com/Sourcehaven-BV/rela/internal/validation"
)

// ValidationFilter specifies which validation rules to run. Multiple
// filters union (OR).
type ValidationFilter struct {
	RuleName   string
	EntityType string
}

// Options configures analysis scope. Scope (when non-nil)
// limits analysis to specific entity IDs; nil means all entities.
type Options struct {
	Scope map[string]bool
}

// DuplicateGroup represents entities with the same normalized title.
type DuplicateGroup struct {
	Title    string
	Entities []*entity.Entity
}

// GapResult contains gaps in an ID sequence.
type GapResult struct {
	Prefix  string
	Missing []string
}

// CardinalityViolation represents a cardinality constraint violation.
type CardinalityViolation struct {
	EntityID     string
	RelationType string
	Constraint   string // "min_outgoing", "max_outgoing", "min_incoming", "max_incoming"
	Required     int
	Actual       int
}

// ValidationViolation re-exports validation.Violation so CLI
// consumers don't need to import internal/validation directly.
type ValidationViolation = validation.Violation

// ValidationResult re-exports validation.Result.
type ValidationResult = validation.Result

// ValidationLoadError re-exports validation.LoadError.
type ValidationLoadError = validation.LoadError

// Summary contains counts from all analysis types.
type Summary struct {
	Orphans                int
	Duplicates             int
	Gaps                   int
	Cardinality            int
	PropertyErrors         int
	ValidationErrors       int
	ValidationWarnings     int
	ValidationScriptErrors int
	ValidationLoadErrors   int
}

// Deps is the dependency bundle [New] requires.
//
// Store, Meta, Tracer, LuaReadDeps are mandatory.
// LuaCache is optional (nil disables shared rela.cache.* between
// validation rules).
// FS and Paths are optional: when nil, [Service.FindOrphanedTempFiles]
// returns (nil, nil) — analyses that don't touch the filesystem still
// work.
type Deps struct {
	Store       store.Store
	Meta        *metamodel.Metamodel
	Tracer      tracer.Tracer
	LuaReadDeps lua.ReadDeps
	LuaCache    *lua.Cache
	FS          storage.FS
	Paths       *project.Context
}

// Service is the read-only analysis facade.
type Service struct {
	deps Deps
}

// New constructs a Service. Returns an error if any required
// dependency is nil — CLAUDE.md "constructors reject nil required
// fields". FS and Paths and LuaCache are optional (see Deps).
func New(d Deps) (*Service, error) {
	switch {
	case d.Store == nil:
		return nil, errors.New("analysis: Store is required")
	case d.Meta == nil:
		return nil, errors.New("analysis: Meta is required")
	case d.Tracer == nil:
		return nil, errors.New("analysis: Tracer is required")
	}
	return &Service{deps: d}, nil
}

// --- Orphan analysis ---

// FindOrphansWithScope returns entities with no relations, filtered
// by scope.
//
// Errors from the tracer or per-entity store reads are logged via
// slog.Warn and the impacted entries are skipped — the caller sees
// an under-count rather than a hard failure. This matches the
// existing CLI summary behavior (AnalyzeAll reports `len(orphans)`
// without an error channel). A returning-errors variant is a
// candidate follow-up; not in scope for the package lift.
func (s *Service) FindOrphansWithScope(ctx context.Context, opts Options) []*entity.Entity {
	ids, err := s.deps.Tracer.FindOrphans(ctx)
	if err != nil {
		slog.Warn("analysis: tracer.FindOrphans failed; returning no orphans (results may under-count)", "error", err)
		return nil
	}
	st := s.deps.Store
	out := make([]*entity.Entity, 0, len(ids))
	for _, id := range ids {
		if !inScope(id, opts.Scope) {
			continue
		}
		e, err := st.GetEntity(ctx, id)
		if err != nil {
			slog.Warn("analysis: store.GetEntity failed; orphan skipped (results may under-count)", "id", id, "error", err)
			continue
		}
		out = append(out, e)
	}
	return out
}

// --- Duplicate analysis ---

// FindDuplicates returns groups of entities with similar titles,
// filtered by scope.
func (s *Service) FindDuplicates(ctx context.Context, opts Options) []DuplicateGroup {
	entities := filterByScope(collectEntities(ctx, s.deps.Store, store.EntityQuery{}), opts.Scope)

	titleGroups := make(map[string][]*entity.Entity)
	for _, e := range entities {
		title := normalizeTitle(e.Title())
		if title != "" {
			titleGroups[title] = append(titleGroups[title], e)
		}
	}

	var duplicates []DuplicateGroup
	for _, group := range titleGroups {
		if len(group) > 1 {
			duplicates = append(duplicates, DuplicateGroup{
				Title:    group[0].Title(),
				Entities: group,
			})
		}
	}
	return duplicates
}

// --- Gap analysis ---

// FindGaps returns gaps in ID sequences, filtered by scope. Excludes
// entity types with manual (string) IDs.
func (s *Service) FindGaps(ctx context.Context, opts Options) []GapResult {
	meta := s.deps.Meta
	stringIDPrefixes := make(map[string]bool)
	for _, entityDef := range meta.Entities {
		if entityDef.IsManualID() {
			for _, idPrefix := range entityDef.GetIDPrefixes() {
				prefix := strings.TrimSuffix(idPrefix, "-")
				stringIDPrefixes[prefix] = true
			}
		}
	}

	prefixGroups := make(map[string][]int)
	for _, e := range collectEntities(ctx, s.deps.Store, store.EntityQuery{}) {
		if !inScope(e.ID, opts.Scope) {
			continue
		}
		parsed, err := entity.ParseEntityID(e.ID)
		if err != nil || parsed.Prefix == "" {
			continue
		}
		if stringIDPrefixes[strings.TrimSuffix(parsed.Prefix, "-")] {
			continue
		}
		prefixGroups[parsed.Prefix] = append(prefixGroups[parsed.Prefix], parsed.Number)
	}

	var allGaps []GapResult
	for prefix, numbers := range prefixGroups {
		sort.Ints(numbers)
		var gaps []int
		for i := 1; i < len(numbers); i++ {
			expected := numbers[i-1] + 1
			if numbers[i] != expected {
				for j := expected; j < numbers[i]; j++ {
					gaps = append(gaps, j)
				}
			}
		}
		if len(gaps) > 0 {
			gapStrs := make([]string, len(gaps))
			for i, n := range gaps {
				gapStrs[i] = fmt.Sprintf("%s%03d", prefix, n)
			}
			allGaps = append(allGaps, GapResult{
				Prefix:  prefix,
				Missing: gapStrs,
			})
		}
	}
	return allGaps
}

// --- Cardinality analysis ---

// CheckCardinality checks all cardinality constraints, filtered by
// scope.
func (s *Service) CheckCardinality(ctx context.Context, opts Options) []CardinalityViolation {
	violations := make([]CardinalityViolation, 0) //nolint:prealloc // capacity unknown

	for relName, relDef := range s.deps.Meta.Relations {
		violations = append(violations, s.checkMinOutgoing(ctx, relName, relDef, opts.Scope)...)
		violations = append(violations, s.checkMaxOutgoing(ctx, relName, relDef, opts.Scope)...)
		violations = append(violations, s.checkMinIncoming(ctx, relName, relDef, opts.Scope)...)
		violations = append(violations, s.checkMaxIncoming(ctx, relName, relDef, opts.Scope)...)
	}
	return violations
}

func (s *Service) checkMinOutgoing(
	ctx context.Context, relName string, relDef metamodel.RelationDef, scope map[string]bool,
) []CardinalityViolation {
	if relDef.MinOutgoing == nil || *relDef.MinOutgoing == 0 {
		return nil
	}
	var violations []CardinalityViolation
	for _, sourceType := range relDef.From {
		entities := collectEntities(ctx, s.deps.Store, store.EntityQuery{Type: sourceType})
		for _, e := range filterByScope(entities, scope) {
			count := s.countOutgoingByType(ctx, e.ID, relName)
			if count < *relDef.MinOutgoing {
				violations = append(violations, CardinalityViolation{
					EntityID:     e.ID,
					RelationType: relName,
					Constraint:   "min_outgoing",
					Required:     *relDef.MinOutgoing,
					Actual:       count,
				})
			}
		}
	}
	return violations
}

func (s *Service) checkMaxOutgoing(
	ctx context.Context, relName string, relDef metamodel.RelationDef, scope map[string]bool,
) []CardinalityViolation {
	if relDef.MaxOutgoing == nil {
		return nil
	}
	var violations []CardinalityViolation
	for _, sourceType := range relDef.From {
		entities := collectEntities(ctx, s.deps.Store, store.EntityQuery{Type: sourceType})
		for _, e := range filterByScope(entities, scope) {
			count := s.countOutgoingByType(ctx, e.ID, relName)
			if count > *relDef.MaxOutgoing {
				violations = append(violations, CardinalityViolation{
					EntityID:     e.ID,
					RelationType: relName,
					Constraint:   "max_outgoing",
					Required:     *relDef.MaxOutgoing,
					Actual:       count,
				})
			}
		}
	}
	return violations
}

func (s *Service) checkMinIncoming(
	ctx context.Context, relName string, relDef metamodel.RelationDef, scope map[string]bool,
) []CardinalityViolation {
	if relDef.MinIncoming == nil || *relDef.MinIncoming == 0 {
		return nil
	}
	var violations []CardinalityViolation
	for _, targetType := range relDef.To {
		entities := collectEntities(ctx, s.deps.Store, store.EntityQuery{Type: targetType})
		for _, e := range filterByScope(entities, scope) {
			count := s.countIncomingByType(ctx, e.ID, relName)
			if count < *relDef.MinIncoming {
				relLabel := relName
				if relDef.Inverse != nil && relDef.Inverse.GetID() != "" {
					relLabel = relDef.Inverse.GetID()
				}
				violations = append(violations, CardinalityViolation{
					EntityID:     e.ID,
					RelationType: relLabel,
					Constraint:   "min_incoming",
					Required:     *relDef.MinIncoming,
					Actual:       count,
				})
			}
		}
	}
	return violations
}

func (s *Service) checkMaxIncoming(
	ctx context.Context, relName string, relDef metamodel.RelationDef, scope map[string]bool,
) []CardinalityViolation {
	if relDef.MaxIncoming == nil {
		return nil
	}
	var violations []CardinalityViolation
	for _, targetType := range relDef.To {
		entities := collectEntities(ctx, s.deps.Store, store.EntityQuery{Type: targetType})
		for _, e := range filterByScope(entities, scope) {
			count := s.countIncomingByType(ctx, e.ID, relName)
			if count > *relDef.MaxIncoming {
				relLabel := relName
				if relDef.Inverse != nil && relDef.Inverse.GetID() != "" {
					relLabel = relDef.Inverse.GetID()
				}
				violations = append(violations, CardinalityViolation{
					EntityID:     e.ID,
					RelationType: relLabel,
					Constraint:   "max_incoming",
					Required:     *relDef.MaxIncoming,
					Actual:       count,
				})
			}
		}
	}
	return violations
}

func (s *Service) countOutgoingByType(ctx context.Context, entityID, relName string) int {
	n, _ := s.deps.Store.CountRelations(ctx, store.RelationQuery{
		EntityID:  entityID,
		Direction: store.DirectionOutgoing,
		Type:      relName,
	})
	return n
}

func (s *Service) countIncomingByType(ctx context.Context, entityID, relName string) int {
	n, _ := s.deps.Store.CountRelations(ctx, store.RelationQuery{
		EntityID:  entityID,
		Direction: store.DirectionIncoming,
		Type:      relName,
	})
	return n
}

// --- Custom validations ---

// newValidationService wires a validation service against the
// service's Lua deps + cache. Construction is cheap (rules come from
// Meta on every Check call); the per-call instance avoids cache
// aliasing if a future caller passes a different LuaCache.
func (s *Service) newValidationService() *validation.Service {
	svc := validation.New(s.deps.Meta, s.deps.LuaReadDeps)
	if s.deps.LuaCache != nil {
		return svc.WithCache(s.deps.LuaCache)
	}
	return svc
}

// RunValidations executes all custom validation rules from the
// metamodel, filtered by scope.
func (s *Service) RunValidations(ctx context.Context, opts Options) ValidationResult {
	return s.newValidationService().Check(ctx, collectEntities(ctx, s.deps.Store, store.EntityQuery{}), opts.Scope)
}

// RunValidationsFiltered executes custom validation rules matching
// the given filters. Multiple filters union (OR). An empty
// ValidationFilter matches all rules.
func (s *Service) RunValidationsFiltered(
	ctx context.Context,
	opts Options,
	filters []ValidationFilter,
) ValidationResult {
	svc := s.newValidationService()

	ruleNames := make(map[string]bool)
	for _, filter := range filters {
		for _, rule := range svc.Rules() {
			if matchesFilter(rule, filter) {
				ruleNames[rule.Name] = true
			}
		}
	}

	return svc.CheckRules(ctx, collectEntities(ctx, s.deps.Store, store.EntityQuery{}), opts.Scope, ruleNames)
}

// matchesFilter returns true if the rule matches the filter criteria.
func matchesFilter(rule metamodel.ValidationRule, filter ValidationFilter) bool {
	if filter.RuleName != "" {
		return rule.Name == filter.RuleName
	}
	if filter.EntityType != "" {
		return rule.EntityType == filter.EntityType
	}
	return true
}

// CountValidationsBySeverity returns counts of errors and warnings
// from violations.
func CountValidationsBySeverity(violations []ValidationViolation) (errors, warnings int) {
	return validation.CountBySeverity(violations)
}

// --- Summary ---

// AnalyzeAll runs all analyses and returns a summary of counts.
func (s *Service) AnalyzeAll(ctx context.Context, opts Options) *Summary {
	summary := &Summary{
		Orphans:     len(s.FindOrphansWithScope(ctx, opts)),
		Duplicates:  len(s.FindDuplicates(ctx, opts)),
		Gaps:        len(s.FindGaps(ctx, opts)),
		Cardinality: len(s.CheckCardinality(ctx, opts)),
	}

	for _, pe := range schema.ValidateEntityProperties(ctx, s.deps.Store, s.deps.Meta) {
		if !inScope(pe.EntityID, opts.Scope) {
			continue
		}
		summary.PropertyErrors += len(pe.Errors)
	}

	result := s.RunValidations(ctx, opts)
	summary.ValidationErrors, summary.ValidationWarnings = validation.CountBySeverity(result.Violations)
	summary.ValidationScriptErrors = len(result.ScriptErrors)
	summary.ValidationLoadErrors = len(result.LoadErrors)

	return summary
}

// --- Orphan temp files ---

// FindOrphanedTempFiles returns paths of leftover .new temp files in
// the entities/ and relations/ directories. Returns (nil, nil) when
// the service was constructed without FS + Paths.
func (s *Service) FindOrphanedTempFiles() ([]string, error) {
	if s.deps.FS == nil || s.deps.Paths == nil {
		return nil, nil
	}
	orphaned := make([]string, 0) //nolint:prealloc // capacity unknown
	orphaned = append(orphaned, findTempFilesInDir(s.deps.FS, s.deps.Paths.EntitiesDir)...)
	orphaned = append(orphaned, findTempFilesInDir(s.deps.FS, s.deps.Paths.RelationsDir)...)
	return orphaned, nil
}

// CleanupOrphanedTempFiles removes every orphaned .new temp file.
// Returns the number of files cleaned up. Returns (0, nil) when the
// service was constructed without FS + Paths — same gate as
// [Service.FindOrphanedTempFiles].
func (s *Service) CleanupOrphanedTempFiles() (int, error) {
	orphaned, err := s.FindOrphanedTempFiles()
	if err != nil {
		return 0, err
	}
	for _, path := range orphaned {
		if removeErr := s.deps.FS.Remove(path); removeErr != nil {
			return 0, fmt.Errorf("remove %s: %w", path, removeErr)
		}
	}
	return len(orphaned), nil
}

// findTempFilesInDir walks a directory (recursively) for .new temp files.
func findTempFilesInDir(fs storage.FS, dir string) []string {
	var result []string
	entries, err := fs.ReadDir(dir)
	if err != nil {
		return nil
	}
	for _, entry := range entries {
		name := entry.Name()
		path := dir + "/" + name
		if entry.IsDir() {
			result = append(result, findTempFilesInDir(fs, path)...)
		} else if strings.HasSuffix(name, ".new") {
			result = append(result, path)
		}
	}
	return result
}

// --- helpers ---

// collectEntities iterates the store yielding entities. On iteration
// error, logs and returns the partial slice (callers cannot signal
// errors today — see [Service.FindOrphansWithScope] for the rationale).
func collectEntities(ctx context.Context, s store.Store, q store.EntityQuery) []*entity.Entity {
	out := make([]*entity.Entity, 0)
	for e, err := range s.ListEntities(ctx, q) {
		if err != nil {
			slog.Warn("analysis: store.ListEntities iterator error; results may under-count",
				"type", q.Type, "error", err)
			return out
		}
		out = append(out, e)
	}
	return out
}

// filterByScope returns only entities present in scope. nil scope is
// a pass-through.
func filterByScope(entities []*entity.Entity, scope map[string]bool) []*entity.Entity {
	if scope == nil {
		return entities
	}
	result := make([]*entity.Entity, 0, len(entities))
	for _, e := range entities {
		if scope[e.ID] {
			result = append(result, e)
		}
	}
	return result
}

// inScope returns true if entityID is in scope (or scope is nil).
func inScope(entityID string, scope map[string]bool) bool {
	if scope == nil {
		return true
	}
	_, exists := scope[entityID]
	return exists
}

// normalizeTitle normalizes a title for duplicate detection.
func normalizeTitle(s string) string {
	s = strings.ToLower(s)
	s = strings.TrimSpace(s)
	fields := strings.Fields(s)
	return strings.Join(fields, " ")
}
