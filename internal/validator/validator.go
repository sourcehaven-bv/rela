// Package validator provides a Validator service that runs metamodel
// validation rules over a store.
//
// Following the same pattern as tracer and search: validation is
// a separate query service that reads from a store.EntityReader. Smart
// backends (e.g. Postgres with constraints) could implement Validator
// natively. The generic GenericValidator iterates the store and runs each
// rule via a metamodel.Metamodel + validation.Service.
package validator

import (
	"context"
	"iter"
	"log/slog"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/lua"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/validation"
	"github.com/Sourcehaven-BV/rela/internal/validationgraph"
)

// Violation represents a custom validation rule violation.
type Violation struct {
	RuleName    string
	Description string
	Severity    string
	EntityID    string
	EntityType  string
	EntityTitle string
	// Message is the per-entity explanation a Lua rule returned, alongside
	// (never instead of) Description. Empty for non-Lua rules. See
	// validation.Violation.Message.
	Message string
	// Face names the content state that violated, as declared. Empty for a
	// type with no faces. See validation.Violation.Face.
	Face string
}

// RuleViolation is one entity's violation of a rule, with optional
// structured detail about why it fired (e.g. which required headers are
// missing). Detail is nil for violations that carry no structured
// specifics (Lua, then-filter, property rules).
type RuleViolation struct {
	EntityID string
	// Message is the per-entity explanation a Lua rule returned for this
	// entity. Empty for non-Lua rules and for a Lua rule that returned no
	// message; renderers fall back to the rule description alone. See
	// validation.Violation.Message.
	Message string
	// Face names the content state that violated, as declared. Empty for a
	// type with no faces. See validation.Violation.Face.
	Face   string
	Detail []string
}

// RuleResult is the full per-rule outcome surfaced to consumers that
// want to render Lua-script failures (rule did not run) distinctly
// from violations (rule ran and found a problem with an entity).
type RuleResult struct {
	// Violations is the list of entities that violated the rule, each
	// with optional structured detail.
	Violations []RuleViolation
	// ScriptErrors describe Lua failures (compile, runtime, timeout,
	// contract). Each carries enough context (path, line, source
	// slice) to render an actionable message.
	ScriptErrors []*lua.ScriptError
	// LoadErrors describe lua_file: rules whose script could not be
	// opened.
	LoadErrors []LoadError
}

// LoadError records a `lua_file:` rule whose script could not be
// loaded. Mirrors validation.LoadError without exposing the
// validation package directly.
type LoadError struct {
	RuleName string
	Message  string
}

// Validator runs custom metamodel validation rules over a store.
type Validator interface {
	// CheckRule returns IDs of entities that violate the given rule.
	// Lua-script failures (rule did not run) are dropped; consumers
	// that need to render those should use CheckRuleFull instead.
	CheckRule(ctx context.Context, rule metamodel.ValidationRule) ([]string, error)

	// CheckRuleFull returns the full per-rule result including
	// ScriptErrors and LoadErrors so the caller can distinguish
	// "rule ran, here are the violations" from "rule did not run."
	CheckRuleFull(ctx context.Context, rule metamodel.ValidationRule) (RuleResult, error)

	// CheckAll runs all rules from the metamodel and returns all violations.
	CheckAll(ctx context.Context) ([]Violation, error)
}

// EntityLister is the narrow read surface the validator needs to load the
// candidate entities it validates — just ListEntities. Consumer-side interface
// (not store.EntityReader) so a GATED reader that exposes only reads can back
// the validator: analyze wires a per-principal gated reader here so a rule's
// candidate set is the requester's visible slice (TKT-3FL2S6). store.Store and
// visibility.ScriptReader both satisfy it structurally.
type EntityLister interface {
	ListEntities(ctx context.Context, q store.EntityQuery) iter.Seq2[*entity.Entity, error]
}

// GenericValidator implements Validator by reading from a store.
type GenericValidator struct {
	r    EntityLister
	meta *metamodel.Metamodel
	svc  *validation.Service
}

var _ Validator = (*GenericValidator)(nil)

// New creates a Validator backed by an EntityLister and a metamodel.
// deps provides read-only Lua access for validation rules that use Lua scripts.
// deps.ProjectRoot is used to resolve lua_file paths from validations/.
//
// Relation-cardinality gates (`relations:`) read through deps.VisibleReader,
// so they count what that reader can see — the same reader, and therefore the
// same visibility, the rest of the rule evaluation uses. A nil VisibleReader
// leaves the graph unwired, and the engine then reports those constraints as
// unevaluable rather than satisfied.
func New(r EntityLister, meta *metamodel.Metamodel, deps lua.ReadDeps) *GenericValidator {
	svc := validation.New(meta, deps)
	// Mirrors analysis.newValidationService: both entry points into a
	// Service must wire this the same way, or a `relations:` gate would
	// mean something different depending on which one ran it.
	//
	// A failure to build it is logged rather than returned, because a
	// graph-less Service is already fail-LOUD: every relation constraint
	// reports as unevaluable instead of passing. The log is what turns
	// "every gate on this deployment errors" into something an operator can
	// diagnose without reading the evaluator.
	if g, err := validationgraph.New(deps.VisibleReader); err != nil {
		slog.Warn("validator: relation-cardinality gates unavailable; they will report as unevaluable",
			"error", err)
	} else {
		svc = svc.WithGraph(g)
	}
	return &GenericValidator{
		r:    r,
		meta: meta,
		svc:  svc,
	}
}

// CheckRule returns IDs of entities that violate the given rule.
// Lua-script failures are dropped; use CheckRuleFull to see them.
func (v *GenericValidator) CheckRule(ctx context.Context, rule metamodel.ValidationRule) ([]string, error) {
	full, err := v.CheckRuleFull(ctx, rule)
	if err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(full.Violations))
	for _, vi := range full.Violations {
		ids = append(ids, vi.EntityID)
	}
	return ids, nil
}

// CheckRuleFull runs a single rule and returns the full result —
// violation entity IDs, ScriptErrors, and LoadErrors — so callers
// that render in a UI can distinguish "rule did not run" from "rule
// ran and flagged an entity."
func (v *GenericValidator) CheckRuleFull(
	ctx context.Context,
	rule metamodel.ValidationRule,
) (RuleResult, error) {
	candidates, err := v.loadCandidates(ctx, rule.EntityType)
	if err != nil {
		return RuleResult{}, err
	}

	result := v.svc.CheckRule(ctx, rule, candidates, nil)
	out := RuleResult{
		Violations:   make([]RuleViolation, 0, len(result.Violations)),
		ScriptErrors: result.ScriptErrors,
	}
	for _, vi := range result.Violations {
		out.Violations = append(out.Violations, RuleViolation{
			EntityID: vi.EntityID,
			Message:  vi.Message,
			Face:     vi.Face,
			Detail:   vi.Detail,
		})
	}
	for _, le := range result.LoadErrors {
		out.LoadErrors = append(out.LoadErrors, LoadError{
			RuleName: le.RuleName,
			Message:  le.Message,
		})
	}
	return out, nil
}

// CheckAll runs all rules from the metamodel and returns all violations.
func (v *GenericValidator) CheckAll(ctx context.Context) ([]Violation, error) {
	candidates, err := v.loadCandidates(ctx, "")
	if err != nil {
		return nil, err
	}

	raw := v.svc.Check(ctx, candidates, nil)
	out := make([]Violation, 0, len(raw.Violations))
	for _, r := range raw.Violations {
		out = append(out, Violation{
			RuleName:    r.RuleName,
			Description: r.Description,
			Message:     r.Message,
			Severity:    r.Severity,
			EntityID:    r.EntityID,
			Face:        r.Face,
			EntityTitle: r.EntityTitle,
		})
	}
	return out, nil
}

// loadCandidates loads entities of the given type from the store.
//
// Entities whose body is unreadable (e.g. git-crypt encrypted, no key
// in the local working tree) are skipped: their property values are not
// available, so applying property-driven rules to them would produce
// false-positive "required field missing" violations. They remain
// visible to other consumers (search, data-entry); only the validator
// cannot meaningfully evaluate rules against them.
// # Every content state, not just the bare row
//
// The query sets AllStates, so each FACE of an entity is validated as its own
// row. Without it (the zero value means default-state rows only) a rule
// evaluated the bare row and silently never ran against any other state — so a
// published face could be missing a required property while `rela validate`
// reported a clean run, which is worse than no check because it is a claim
// (TKT-4Y6CMV).
//
// Every row of a state family conforms to the same schema, so validating each
// one is the same question asked of each. Choosing ONE state to validate would
// be world resolution, a read-path concern that has no business here — and the
// store forbids combining AllStates with a World for exactly that reason.
func (v *GenericValidator) loadCandidates(ctx context.Context, entityType string) ([]*entity.Entity, error) {
	q := store.EntityQuery{AllStates: true}
	if entityType != "" {
		q.Type = entityType
	}

	out := make([]*entity.Entity, 0)
	for e, err := range v.r.ListEntities(ctx, q) {
		if err != nil {
			return nil, err
		}
		if e.IsLocked() {
			continue
		}
		out = append(out, e)
	}
	return out, nil
}
