// Package validation provides custom validation rule checking for entities.
// It encapsulates the logic for evaluating metamodel validation rules against
// entity sets, supporting scoped validation for view-based analysis.
package validation

import (
	"context"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/filter"
	"github.com/Sourcehaven-BV/rela/internal/lua"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// Violation represents a custom validation rule violation.
type Violation struct {
	RuleName    string
	Description string
	Severity    string // "error" or "warning"
	EntityID    string
	EntityTitle string
}

// Result aggregates the outputs of a validation pass: rule findings,
// Lua failures, and load-time failures. The three slices are
// semantically distinct:
//
//   - Violations: rule ran successfully and found a problem with an
//     entity.
//   - ScriptErrors: a Lua rule failed to run (compile, runtime,
//     timeout, or contract violation like wrong return type) — the
//     rule did NOT report on entities.
//   - LoadErrors: a `lua_file:` rule's script could not be opened
//     (config-level failure, no Lua VM ever ran).
//
// Splitting these prevents the CLI from conflating "entity violated
// rule" (a finding) with "rule did not run" (an environment problem).
type Result struct {
	Violations   []Violation
	ScriptErrors []*lua.ScriptError
	LoadErrors   []LoadError
}

// LoadError records a `lua_file:` rule whose script could not be
// loaded. Message is already sanitized by loadLuaScript (no system
// paths leaked).
type LoadError struct {
	RuleName string
	Message  string
}

// HasErrors reports whether any rule failed to execute (Lua failure
// or script-load failure). Unrelated to whether Violations were
// found — a clean run with violations still has HasErrors() == false.
func (r Result) HasErrors() bool {
	return len(r.ScriptErrors) > 0 || len(r.LoadErrors) > 0
}

// Service validates entities against custom metamodel rules.
type Service struct {
	deps  lua.ReadDeps
	cache *lua.Cache
}

// New creates a validation service for the given metamodel.
// deps provides read-only lua access for rules that use Lua scripts.
// The ProjectRoot field of deps is used to resolve lua_file paths from
// validations/. The supplied metamodel is authoritative — it overwrites
// deps.Meta so the Go-side rule evaluator and the Lua-side filter helpers
// cannot disagree about which schema is in effect.
func New(meta *metamodel.Metamodel, deps lua.ReadDeps) *Service {
	deps.Meta = meta
	return &Service{deps: deps}
}

// WithCache wires a shared Lua cache into validation runs so
// rela.cache.* inside validation scripts is functional and namespaced
// per-rule. Zero-or-nil cache leaves validation runtimes un-cached.
func (s *Service) WithCache(c *lua.Cache) *Service {
	s.cache = c
	return s
}

// Rules returns the validation rules from the metamodel.
func (s *Service) Rules() []metamodel.ValidationRule {
	return s.deps.Meta.Validations
}

// Check runs all validation rules against the given entities.
// If scope is non-nil, only entities in scope are checked.
//
// ctx is propagated into Lua execution: canceling it interrupts an
// in-flight rule. Pass cmd.Context() (cobra) or r.Context() (HTTP) for
// real cancellation; tests use context.Background.
func (s *Service) Check(ctx context.Context, entities []*entity.Entity, scope map[string]bool) Result {
	return s.CheckRules(ctx, entities, scope, nil)
}

// CheckRules runs validation rules against the given entities.
// If ruleNames is nil, all rules are run. Otherwise, only rules in the set are run.
// If scope is non-nil, only entities in scope are checked.
func (s *Service) CheckRules(
	ctx context.Context,
	entities []*entity.Entity,
	scope, ruleNames map[string]bool,
) Result {
	var result Result

	for _, rule := range s.deps.Meta.Validations {
		// Bail out early on parent ctx cancellation so we don't
		// construct N more runtimes that would all fail-fast on the
		// already-dead context. Cheap: ctx.Err() is a single load.
		if ctx.Err() != nil {
			break
		}
		// Skip rules not in the filter set (if filter is specified)
		if ruleNames != nil && !ruleNames[rule.Name] {
			continue
		}

		ruleResult := s.CheckRule(ctx, rule, entities, scope)
		result.Violations = append(result.Violations, ruleResult.Violations...)
		result.ScriptErrors = append(result.ScriptErrors, ruleResult.ScriptErrors...)
		result.LoadErrors = append(result.LoadErrors, ruleResult.LoadErrors...)
	}

	return result
}

// CountBySeverity returns counts of errors and warnings from violations.
func CountBySeverity(violations []Violation) (errors, warnings int) {
	for _, v := range violations {
		if v.Severity == "error" {
			errors++
		} else {
			warnings++
		}
	}
	return
}

// CheckRule checks a single rule against entities.
//
// Lua is hoisted to one runtime per rule (not per (rule, entity)) so
// module-local Lua memoization persists across entities and the
// rule's `*lua.Cache` namespace is reused. The runtime is constructed
// at most once per CheckRule invocation; rules without Lua never
// build one.
func (s *Service) CheckRule(
	ctx context.Context,
	rule metamodel.ValidationRule,
	entities []*entity.Entity,
	scope map[string]bool,
) Result {
	// Parse filters
	whenFilters, err := filter.ParseAll(rule.When)
	if err != nil {
		return Result{}
	}
	thenFilters, err := filter.ParseAll(rule.Then)
	if err != nil {
		return Result{}
	}

	// Filter candidates
	candidates := s.filterCandidates(entities, scope, rule.EntityType)

	// If the rule uses Lua, build a single runtime and reuse it across
	// every entity. luaCtx is nil when the rule has no Lua at all.
	var luaCtx *luaRuleContext
	var result Result
	hasLua := rule.Lua != "" || rule.LuaFile != ""
	if hasLua {
		built, loadErr := s.buildLuaRuleContext(ctx, rule)
		if loadErr != nil {
			result.LoadErrors = append(result.LoadErrors, *loadErr)
			// Lua won't run for this rule; non-Lua checks still apply
			// (when/then/content) so candidates are still iterated below.
		} else if built != nil {
			luaCtx = built
		}
	}
	// Close the active runtime (if any) on exit. Reassigned below when
	// the runtime is rebuilt after a ScriptError to flush any partial
	// state (half-built coroutines, leaked locals) the rule may have
	// left behind.
	defer func() {
		if luaCtx != nil {
			luaCtx.runtime.Close()
		}
	}()

	for _, e := range candidates {
		// The Lua path under checkEntityAgainstRule runs through luaCtx.runtime,
		// which was built with lua.WithContext(ctx); applyTimeout derives the
		// per-entity budget from that cached parent ctx. contextcheck can't
		// follow that flow across the gopher-lua SetContext boundary.
		//nolint:contextcheck // ctx threaded via WithContext on luaCtx.runtime
		entityResult := s.checkEntityAgainstRule(e, rule, whenFilters, thenFilters, luaCtx)
		result.Violations = append(result.Violations, entityResult.Violations...)
		result.ScriptErrors = append(result.ScriptErrors, entityResult.ScriptErrors...)

		// If Lua errored on this entity, the runtime may be in an
		// undefined state (partial coroutines, half-mutated globals).
		// Close it and rebuild a fresh one for the next entity so the
		// next rule invocation cannot observe leaked state. Cost is
		// bounded — only paid when errors actually occur.
		if hasLua && luaCtx != nil && len(entityResult.ScriptErrors) > 0 {
			luaCtx.runtime.Close()
			luaCtx = nil
			rebuilt, loadErr := s.buildLuaRuleContext(ctx, rule)
			if loadErr != nil {
				// Script vanished mid-iteration — surface as LoadError
				// and skip Lua for remaining entities.
				result.LoadErrors = append(result.LoadErrors, *loadErr)
			} else if rebuilt != nil {
				luaCtx = rebuilt
			}
		}
	}
	return result
}

// filterCandidates filters entities by scope and entity type.
func (s *Service) filterCandidates(
	entities []*entity.Entity,
	scope map[string]bool,
	entityType string,
) []*entity.Entity {
	var candidates []*entity.Entity
	for _, e := range entities {
		if scope != nil {
			if _, ok := scope[e.ID]; !ok {
				continue
			}
		}
		if entityType == "" || e.Type == entityType {
			candidates = append(candidates, e)
		}
	}
	return candidates
}

// entityResult is the per-entity outcome of CheckRule. It carries
// Violations and ScriptErrors but never LoadErrors (those are
// per-rule, hoisted out of the entity loop).
type entityResult struct {
	Violations   []Violation
	ScriptErrors []*lua.ScriptError
}

// checkEntityAgainstRule checks if an entity violates the given rule.
// luaCtx, when non-nil, supplies the per-rule runtime + envelope path
// for Lua execution; passing nil disables the Lua path even when the
// rule defines lua/lua_file (used when the script failed to load).
func (s *Service) checkEntityAgainstRule(
	e *entity.Entity,
	rule metamodel.ValidationRule,
	whenFilters, thenFilters []*filter.Filter,
	luaCtx *luaRuleContext,
) entityResult {
	entityDef, ok := s.deps.Meta.GetEntityDef(e.Type)
	if !ok {
		return entityResult{}
	}

	rec := filter.Record{ID: e.ID, Type: e.Type, Properties: e.Properties}

	// Check 'when' conditions - if they don't match, rule doesn't apply
	if len(whenFilters) > 0 {
		matches, err := filter.MatchAll(rec, whenFilters, entityDef, s.deps.Meta)
		if err != nil || !matches {
			return entityResult{}
		}
	}

	// Check 'then' conditions - if they don't satisfy, it's a violation
	if len(thenFilters) > 0 {
		satisfies, err := filter.MatchAll(rec, thenFilters, entityDef, s.deps.Meta)
		if err != nil || !satisfies {
			return entityResult{Violations: []Violation{newViolation(rule, e, rule.Description)}}
		}
	}

	// Check Lua validation rules. A Lua failure (scriptErr) is
	// surfaced but does NOT suppress the content check that follows,
	// so a rule defining both `lua:` and `content:` still flags
	// content violations even when the Lua portion errored. Lua
	// violations short-circuit content, matching pre-change
	// behavior where the rule reported once per entity.
	var out entityResult
	if luaCtx != nil {
		luaViolations, scriptErr := s.runLuaForEntity(e, rule, luaCtx)
		if scriptErr != nil {
			out.ScriptErrors = append(out.ScriptErrors, scriptErr)
		} else if len(luaViolations) > 0 {
			out.Violations = append(out.Violations, luaViolations...)
			return out
		}
	}

	// Check content rules
	if rule.Content != nil && !CheckContentRule(e.Content, rule.Content) {
		out.Violations = append(out.Violations, newViolation(rule, e, rule.Description))
	}

	return out
}

// newViolation constructs a Violation tagged with the rule's metadata.
// description overrides rule.Description for Lua-sourced violations
// (which carry their own custom messages).
func newViolation(rule metamodel.ValidationRule, e *entity.Entity, description string) Violation {
	return Violation{
		RuleName:    rule.Name,
		Description: description,
		Severity:    rule.GetSeverity(),
		EntityID:    e.ID,
		EntityTitle: e.Title(),
	}
}

// runLuaForEntity runs the per-rule Lua against a single entity, using
// the runtime built once in CheckRule. Returns the violations parsed
// from the Lua return value, or a *lua.ScriptError if the Lua failed
// (compile, runtime, timeout, contract violation).
func (s *Service) runLuaForEntity(
	e *entity.Entity, rule metamodel.ValidationRule, luaCtx *luaRuleContext,
) ([]Violation, *lua.ScriptError) {
	luaViolations, scriptErr := s.validateLuaWithRuntime(e, rule, luaCtx)
	if scriptErr != nil {
		return nil, scriptErr
	}
	if len(luaViolations) == 0 {
		return nil, nil
	}
	violations := make([]Violation, len(luaViolations))
	for i, lv := range luaViolations {
		violations[i] = Violation{
			RuleName:    rule.Name,
			Description: lv.Message,
			Severity:    lv.Severity,
			EntityID:    e.ID,
			EntityTitle: e.Title(),
		}
	}
	return violations, nil
}
