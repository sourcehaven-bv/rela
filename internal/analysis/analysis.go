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

// UniqueViolation represents a group of entities of the same type that
// share a value for a property declared `unique: true` in the metamodel
// — i.e. a natural-key collision the write path would reject today, but
// which may already exist in data that predates the constraint. Reported
// by [Service.FindUniqueViolations] so an operator can find and fix
// pre-existing duplicates before (or after) enabling `unique: true`.
type UniqueViolation struct {
	EntityType string
	Property   string
	Value      string
	Entities   []*entity.Entity
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
	UniqueViolations       int
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

// FindUniqueViolations returns groups of same-type entities that share a
// non-empty value for a property declared `unique: true`, filtered by
// scope. Each returned group has at least two entities. Results are
// sorted by (entity type, property, value) for stable output.
//
// This is the read-side companion to the write-path unique constraint
// (see internal/entitymanager checkUniqueProperties): the write path
// rejects NEW duplicates, this surfaces ones that already exist — e.g.
// after an operator adds `unique: true` to a property whose data already
// contains collisions, which the constraint does not retroactively clean.
// List properties are skipped (a natural key is a scalar), matching the
// write-path check.
func (s *Service) FindUniqueViolations(ctx context.Context, opts Options) []UniqueViolation {
	// (type, property) pairs the metamodel declares unique + non-list.
	type uniqueProp struct{ entityType, property string }
	var uniqueProps []uniqueProp
	for typeName, def := range s.deps.Meta.Entities {
		for propName, pd := range def.PropertyDefs() {
			if pd.Unique && !pd.List {
				uniqueProps = append(uniqueProps, uniqueProp{typeName, propName})
			}
		}
	}
	if len(uniqueProps) == 0 {
		return nil
	}

	entities := filterByScope(collectEntities(ctx, s.deps.Store, store.EntityQuery{}), opts.Scope)

	// Group by (type, property, value); a group with >1 entity is a
	// violation. valueGroups keyed on the uniqueProp then the value.
	type groupKey struct{ up uniqueProp }
	valueGroups := make(map[groupKey]map[string][]*entity.Entity)
	for _, e := range entities {
		for _, up := range uniqueProps {
			if e.Type != up.entityType {
				continue
			}
			v := e.GetString(up.property)
			if v == "" {
				continue // empty values are exempt, per the write-path check
			}
			k := groupKey{up}
			if valueGroups[k] == nil {
				valueGroups[k] = make(map[string][]*entity.Entity)
			}
			valueGroups[k][v] = append(valueGroups[k][v], e)
		}
	}

	var violations []UniqueViolation
	for k, byValue := range valueGroups {
		for value, group := range byValue {
			if len(group) > 1 {
				violations = append(violations, UniqueViolation{
					EntityType: k.up.entityType,
					Property:   k.up.property,
					Value:      value,
					Entities:   group,
				})
			}
		}
	}
	sort.Slice(violations, func(i, j int) bool {
		if violations[i].EntityType != violations[j].EntityType {
			return violations[i].EntityType < violations[j].EntityType
		}
		if violations[i].Property != violations[j].Property {
			return violations[i].Property < violations[j].Property
		}
		return violations[i].Value < violations[j].Value
	})
	return violations
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

// cardinalitySpec is one direction of a relation's cardinality
// constraints: the subject population (which entity types are checked),
// the count direction, the min/max bounds with their constraint labels,
// and the relation label reported on violations (the inverse id for the
// incoming side, when declared).
//
// This is the single seam world-awareness (TKT-9KZGJO) will thread
// through: subject population, counting scope, and violation identity
// each have exactly one home — the spec, [Service.countRelations], and
// the two emit passes in [Service.checkCardinality] (TKT-RNBLAC).
type cardinalitySpec struct {
	relName       string // metamodel relation name — the count query key
	direction     store.Direction
	subjectTypes  []string // relDef.From (outgoing) / relDef.To (incoming)
	minBound      *int     // nil or 0 disables the min check
	maxBound      *int     // nil disables the max check; 0 forbids any edge
	minConstraint string
	maxConstraint string
	relationLabel string // violation display label; inverse id on the incoming side
}

// CheckCardinality checks all cardinality constraints, filtered by
// scope.
//
// A store error fails the run loudly: the first failing count aborts
// with a wrapped error and NO violations. Reporting around a failed
// count would fabricate violations — a backend outage reads as count 0,
// which for a min bound looks exactly like missing relations
// (TKT-RNBLAC). This deliberately diverges from the under-count logging
// of the other analyses (see [Service.FindOrphansWithScope]): those can
// only miss findings, a failed count invents them.
func (s *Service) CheckCardinality(ctx context.Context, opts Options) ([]CardinalityViolation, error) {
	// Non-nil even when empty: JSON callers serialize Details as [], not null.
	violations := make([]CardinalityViolation, 0)

	for relName, relDef := range s.deps.Meta.Relations {
		incomingLabel := relName
		if relDef.Inverse != nil && relDef.Inverse.GetID() != "" {
			incomingLabel = relDef.Inverse.GetID()
		}
		specs := [2]cardinalitySpec{
			{
				relName: relName, direction: store.DirectionOutgoing, subjectTypes: relDef.From,
				minBound: relDef.MinOutgoing, maxBound: relDef.MaxOutgoing,
				minConstraint: "min_outgoing", maxConstraint: "max_outgoing",
				relationLabel: relName,
			},
			{
				relName: relName, direction: store.DirectionIncoming, subjectTypes: relDef.To,
				minBound: relDef.MinIncoming, maxBound: relDef.MaxIncoming,
				minConstraint: "min_incoming", maxConstraint: "max_incoming",
				relationLabel: incomingLabel,
			},
		}
		for _, spec := range specs {
			v, err := s.checkCardinality(ctx, spec, opts.Scope)
			if err != nil {
				return nil, err
			}
			violations = append(violations, v...)
		}
	}
	return violations, nil
}

// checkCardinality evaluates one direction of one relation. Each
// subject is scanned and counted once; min violations are then emitted
// before max violations (two passes over the cached counts) so the
// output order matches the historical per-constraint grouping.
func (s *Service) checkCardinality(
	ctx context.Context, spec cardinalitySpec, scope map[string]bool,
) ([]CardinalityViolation, error) {
	minActive := spec.minBound != nil && *spec.minBound > 0
	maxActive := spec.maxBound != nil
	if !minActive && !maxActive {
		return nil, nil
	}
	dirWord := "outgoing"
	if spec.direction == store.DirectionIncoming {
		dirWord = "incoming"
	}

	// Buffering every (subject, count) before emitting is deliberate: the
	// two emit passes below reproduce the historical min-then-max grouped
	// ordering. Collapsing this into a single count-and-emit pass would
	// interleave min and max violations and reorder the output the
	// pinning tests guard.
	type subject struct {
		id    string
		count int
	}
	var subjects []subject
	for _, subjectType := range spec.subjectTypes {
		entities := collectEntities(ctx, s.deps.Store, store.EntityQuery{Type: subjectType})
		for _, e := range filterByScope(entities, scope) {
			count, err := s.countRelations(ctx, e.ID, spec.relName, spec.direction)
			if err != nil {
				return nil, fmt.Errorf("analysis: count %s %q relations of %s: %w", dirWord, spec.relName, e.ID, err)
			}
			subjects = append(subjects, subject{id: e.ID, count: count})
		}
	}

	var violations []CardinalityViolation
	if minActive {
		for _, sub := range subjects {
			if sub.count < *spec.minBound {
				violations = append(violations, CardinalityViolation{
					EntityID:     sub.id,
					RelationType: spec.relationLabel,
					Constraint:   spec.minConstraint,
					Required:     *spec.minBound,
					Actual:       sub.count,
				})
			}
		}
	}
	if maxActive {
		for _, sub := range subjects {
			if sub.count > *spec.maxBound {
				violations = append(violations, CardinalityViolation{
					EntityID:     sub.id,
					RelationType: spec.relationLabel,
					Constraint:   spec.maxConstraint,
					Required:     *spec.maxBound,
					Actual:       sub.count,
				})
			}
		}
	}
	return violations, nil
}

// countRelations counts an entity's relations of one type in one
// direction. Errors propagate — see the [Service.CheckCardinality]
// error policy.
func (s *Service) countRelations(
	ctx context.Context, entityID, relName string, direction store.Direction,
) (int, error) {
	return s.deps.Store.CountRelations(ctx, store.RelationQuery{
		EntityID:  entityID,
		Direction: direction,
		Type:      relName,
	})
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

// AnalyzeAll runs all analyses and returns a summary of counts. A
// cardinality store error fails the whole run — see the
// [Service.CheckCardinality] error policy.
func (s *Service) AnalyzeAll(ctx context.Context, opts Options) (*Summary, error) {
	cardinality, err := s.CheckCardinality(ctx, opts)
	if err != nil {
		return nil, err
	}
	summary := &Summary{
		Orphans:          len(s.FindOrphansWithScope(ctx, opts)),
		Duplicates:       len(s.FindDuplicates(ctx, opts)),
		UniqueViolations: len(s.FindUniqueViolations(ctx, opts)),
		Gaps:             len(s.FindGaps(ctx, opts)),
		Cardinality:      len(cardinality),
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

	return summary, nil
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
