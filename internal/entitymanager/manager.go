package entitymanager

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/autocascade"
	"github.com/Sourcehaven-BV/rela/internal/automation"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/statemachine"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/templating"
)

// TemplateLoader is the narrow consumer-side surface entitymanager
// needs from a templating implementation. The full
// [templating.Templater] interface has five methods; Manager calls
// exactly two. Defining this interface here (CLAUDE.md
// "consumer-side interfaces" rule) lets tests stub two methods
// instead of five, and keeps Manager decoupled from the
// template-generation half of templating's API.
type TemplateLoader interface {
	EntityTemplate(ctx context.Context, entityType, variant string) (*templating.Template, error)
	RelationTemplate(ctx context.Context, relationType string) (*templating.Template, error)
}

// Manager is the production [EntityManager] implementation. It runs
// metamodel validation, automation rules (via [automation.Engine]),
// and dispatches automation cascades through an [autocascade.Runner].
//
// Manager is constructed at each per-command wiring site (cmd/rela,
// cmd/rela-server, cmd/rela-desktop, plus subcommands that need their
// own EntityManager). Consumers depend on a scoped consumer-side
// interface in their own package, not on *Manager directly (see
// CLAUDE.md). The package-level [EntityManager] interface exists for
// transitional reasons and is intentionally narrow.
//
// Pipeline shapes preserved from the pre-decomposition workspace
// implementation (see PLAN-HQ5Y):
//
//   - Create: createCore (validate → write) → automation.Process →
//     apply property changes → re-write if changed → cascade.
//     Engine.Process runs here (not inside Runner) because
//     PropertiesSet must land on the entity and be persisted before
//     cascade dispatches.
//   - Update: validate → engine.Process(if oldEntity != nil) →
//     apply property changes → write → cascade.
//   - Delete: lookup → collect incident relations → delete relations
//     → delete entity. No automation, no cascade.
//   - Rename: validate → write at new ID → rewrite incident relations
//     → delete old. No automation, no cascade, no re-validation.
//   - CreateRelation: fetch endpoints → validate type → check duplicate
//     → apply template → write. No automation.
//   - UpdateRelation: fetch existing → merge properties → MetaUnset →
//     content → write. No automation.
//   - DeleteRelation: delete. No automation.
type Manager struct {
	deps Deps

	// bypassACL marks this Manager as an ELEVATED write handle: its writes
	// skip the ACL deny (TKT-D8T148, the `rela.bypass_acl(...)` path). It is
	// false on the normal Manager and set true ONLY on the throwaway handle
	// returned by [Manager.elevated]. Carrying elevation on the object (not
	// the context) is what makes it leak-proof: a nested cascade an elevated
	// write triggers re-dispatches with the gated Manager (see the cascade
	// dispatch sites, which pass `m.gated()` as the Mutator), so elevation
	// never propagates to descendant writes.
	bypassACL bool
}

// elevated returns a throwaway Manager handle whose writes skip the ACL deny.
// Shares all deps with the receiver; differs only in bypassACL. Reserved for
// the script-runner's elevated write handle (rela.bypass_acl). NOT exported as
// a general capability — callers obtain it only through the autocascade
// Mutator seam for an allow_acl_bypass action.
func (m *Manager) elevated() *Manager {
	return &Manager{deps: m.deps, bypassACL: true}
}

// Elevated satisfies [autocascade.ElevatedProvider] (TKT-D8T148): it hands the
// script runner an elevated Mutator for an `allow_acl_bypass` action. Exported
// only to cross the package boundary to the script runner; the returned handle
// bypasses the ACL deny but preserves the real principal in audit and never
// propagates elevation into nested cascades (it dispatches via gated()).
func (m *Manager) Elevated() autocascade.Mutator {
	return m.elevated()
}

// gated returns a non-elevated Manager sharing the receiver's deps. On a normal
// Manager it returns the receiver; on an elevated handle it strips the bypass.
// Used at cascade-dispatch sites so a nested cascade triggered by an elevated
// write runs with normal ACL authority — elevation does not propagate to
// descendants (the leak the ctx-marker approach would have had).
func (m *Manager) gated() *Manager {
	if !m.bypassACL {
		return m
	}
	return &Manager{deps: m.deps, bypassACL: false}
}

// Compile-time assertions: Manager must satisfy both the public
// EntityManager contract and the autocascade.Mutator surface (the
// per-cascade write handle scripted actions receive). A drift in
// either interface surfaces at this type, not at the call sites that
// pass Manager into Request.Mutator or lua.WriteDeps.EntityManager.
var (
	_ EntityManager       = (*Manager)(nil)
	_ autocascade.Mutator = (*Manager)(nil)
)

// Deps is the constructor input for [New]. Using a struct keeps the
// constructor signature stable as new collaborators land (audit,
// principal, policy in subsequent tickets).
type Deps struct {
	// Store is the authoritative persistence layer. Required.
	Store store.Store

	// Meta is the active metamodel. Manager uses it for
	// ValidateEntity (entity writes) and ValidateRelation (relation
	// writes). Required.
	Meta *metamodel.Metamodel

	// Templater applies entity-creation and relation-creation
	// templates. The full [templating.Templater] satisfies this
	// narrow contract structurally. Required (use a no-op in tests).
	Templater TemplateLoader

	// Automations is the rule-evaluation engine. Manager calls it
	// on EventEntityCreated / EventEntityUpdated to discover side
	// effects. Optional: nil disables automation processing.
	Automations *automation.Engine

	// Cascade is the autocascade Runner that orchestrates automation
	// side effects after a write. Required iff Automations is non-nil.
	Cascade *autocascade.Runner

	// ScriptRunner executes scripted automation actions during a
	// cascade. May be nil if no scripted automations are configured;
	// Runner records each scripted action as a per-action error when
	// no ScriptRunner is supplied. Wiring sites that need
	// transport-specific deps (e.g. Lua) construct one with the
	// static read deps at this layer; the per-cascade mutator is
	// supplied by Manager via [autocascade.Request.Mutator] (see
	// internal/script/luascriptrunner.go for the Lua adapter).
	ScriptRunner autocascade.ScriptRunner

	// Audit receives one record per successful entity / relation write.
	// Required. Production wiring passes a [audit.Filesystem]; tests use
	// [audit.NewMemory] or [audit.Nop]. Never substitute a silent nil —
	// the constructor rejects nil so missing audit fails fast at wiring
	// time, not later as silently-dropped forensic data.
	Audit audit.Audit

	// ACL gates every write entry point. Required. Production wiring
	// passes [acl.NopACL] (no acl.yaml) or [acl.Declarative] (acl.yaml
	// present); `rela-server --read-only` injects [acl.ReadOnlyACL].
	// Tests use [acl.NopACL] unless they assert on the deny path.
	// Never substitute a silent nil — the constructor rejects nil so
	// missing ACL fails fast at wiring time, not later as a silently
	// disabled authz gate.
	ACL acl.ACL

	// VersionRecorder captures a synchronous entity version for rename and
	// delete (see [VersionRecorder]). Optional: nil disables synchronous
	// version capture (fs/mem builds, and the postgres build's create/update
	// versions are handled by the store's periodic sweep, not this hook).
	VersionRecorder VersionRecorder

	// RelationVersionRecorder captures a synchronous relation version for
	// relation delete (explicit and entity-cascade) and endpoint rename (see
	// [RelationVersionRecorder]). Optional: nil disables it (fs/mem builds; the
	// postgres build's relation create/update versions come from the sweep).
	RelationVersionRecorder RelationVersionRecorder

	// Transitions enforces enum state machines on the write path (TKT-E4LW2):
	// transition legality, guard permissions, and preconditions. Required so
	// no write path can silently skip the machine — but a metamodel with no
	// transitions compiles to an empty enforcer whose checks are no-ops, so
	// "required" costs nothing when the feature is unused. Constructed once at
	// startup by [statemachine.Compile] and injected; the Manager never
	// re-derives it from the metamodel.
	Transitions TransitionEnforcer

	// TransitionGuard answers the guard question for a state-machine
	// transition (does the ctx principal hold permission P for the subject).
	// May be nil: a nil guard makes every guarded edge fail closed, which is
	// the safe default when no ACL-backed guard was wired. Production wiring
	// supplies an adapter over the ACL; the direct-CLI/no-policy case is
	// handled inside that adapter (it allows when there is no policy).
	TransitionGuard statemachine.Guard

	// TransitionGraph answers has_relation/count_relations for a transition
	// `when:` precondition. May be nil when no precondition needs the graph;
	// a `when:` that queries the graph then evaluates against an empty graph.
	TransitionGraph statemachine.GraphLookup
}

// TransitionEnforcer is the narrow contract the Manager needs from the
// compiled state machines: enforce an update (old→new) and a create's entry
// value. Defined at the call site (CLAUDE.md consumer-side interfaces);
// [*statemachine.Set] satisfies it.
type TransitionEnforcer interface {
	EnforceUpdate(
		ctx context.Context, old, updated *entity.Entity,
		guard statemachine.Guard, lookup statemachine.GraphLookup,
	) error
	EnforceCreate(ctx context.Context, e *entity.Entity) error
}

// New constructs a Manager and validates required collaborators.
func New(d Deps) (*Manager, error) {
	if d.Store == nil {
		return nil, errors.New("entitymanager: New: Store is required")
	}
	if d.Meta == nil {
		return nil, errors.New("entitymanager: New: Meta is required")
	}
	if d.Templater == nil {
		return nil, errors.New("entitymanager: New: Templater is required")
	}
	if d.Audit == nil {
		return nil, errors.New("entitymanager: New: Audit is required (use audit.Nop{} to opt out)")
	}
	if d.ACL == nil {
		return nil, errors.New("entitymanager: New: ACL is required (use acl.NopACL{} to opt out)")
	}
	if d.Transitions == nil {
		return nil, errors.New(
			"entitymanager: New: Transitions is required (use statemachine.Compile; an empty set is a no-op)")
	}
	if (d.Automations == nil) != (d.Cascade == nil) {
		return nil, errors.New(
			"entitymanager: New: Automations and Cascade must be supplied together (both non-nil or both nil)",
		)
	}
	return &Manager{deps: d}, nil
}

// authorizeAndAudit consults the ACL and, on deny, records a
// `denied-write` audit row and returns [*acl.ForbiddenError]. On allow,
// returns nil and the caller proceeds. Called as the first
// non-validation step in every write entry point.
//
// The denied-write audit happens regardless of audit backend
// (Filesystem / Memory / Nop) — forensic posture demands recording
// what was attempted, not just what landed.
//
// When this Manager is an elevated handle (`m.bypassACL` — TKT-D8T148: a
// `rela.bypass_acl(...)` write), the ACL deny is SKIPPED: the write is allowed
// regardless of the principal's grants. Elevation is a property of WHICH
// Manager you hold, not of the context — see [Manager.elevated]. The real
// principal is still preserved (`principal.From(ctx)`, unchanged) and the
// bypass is recorded (recordACLBypass) so the audit trail is unambiguous about
// which writes were elevated and on whose behalf. We do NOT recordDeniedWrite
// on the bypass path — there is no denial to record.
func (m *Manager) authorizeAndAudit(ctx context.Context, req acl.WriteRequest) error {
	if m.bypassACL {
		m.recordACLBypass(ctx, req)
		return nil
	}
	decision := m.deps.ACL.AuthorizeWrite(ctx, req)
	if decision.Allow {
		return nil
	}
	m.recordDeniedWrite(ctx, decision, req)
	return &acl.ForbiddenError{Decision: decision}
}

// mapTransitionError translates a state-machine enforcement error into the
// entitymanager's wire-facing error shape (TKT-E4LW2). A guard denial becomes
// an [*acl.ForbiddenError] (RuleKind "transition-guard") so it flows through
// the same 403 path — and audit row — as any other authorization denial;
// legality and precondition failures pass through unchanged and surface as 422
// validation-class errors at the HTTP boundary. Returns nil for a nil input.
func (m *Manager) mapTransitionError(ctx context.Context, subject acl.Subject, err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, statemachine.ErrGuardDenied) {
		decision := acl.Decision{
			Allow:    false,
			RuleKind: "transition-guard",
			RuleID:   "-",
			Reason:   err.Error(),
		}
		m.recordDeniedWrite(ctx, decision, acl.WriteRequest{Op: acl.OpUpdate, Subject: subject})
		return &acl.ForbiddenError{Decision: decision}
	}
	return err
}

// recordACLBypass emits one audit row for an elevated (rela.bypass_acl) write.
// It preserves the real triggering principal (so "who really caused this" is
// always answerable, like ruid under sudo) and marks the row acl_bypass=true
// so forensic queries can isolate elevated writes. The op stays the genuine
// write op; the bypass marker rides the Summary alongside the existing
// triggered_by=automation:<name>.
func (m *Manager) recordACLBypass(ctx context.Context, req acl.WriteRequest) {
	var subject *audit.Subject
	switch s := req.Subject.(type) {
	case acl.RelationSubject:
		subject = &audit.Subject{Kind: "relation", RelationType: s.Type, FromID: s.FromID}
	case acl.EntitySubject:
		subject = &audit.Subject{Kind: "entity", Type: s.Type, ID: s.ID}
	}
	m.deps.Audit.Record(audit.Record{
		Time:        time.Now().UTC(),
		Op:          audit.OpACLBypass,
		Subject:     subject,
		Principal:   principal.From(ctx),
		TriggeredBy: audit.TriggeredByFrom(ctx),
		Summary:     fmt.Sprintf("acl_bypass=true op=%s", req.Op),
	})
}

// recordDeniedWrite emits one audit row describing the refused
// attempt. Subject names the would-be target (entity or relation);
// Summary carries the deny rule_kind / rule_id / reason and the
// attempted op so jq filters can ask "what did Alice try to do?".
func (m *Manager) recordDeniedWrite(ctx context.Context, d acl.Decision, req acl.WriteRequest) {
	// RR-79HD: surface the target ID (entity ID, or relation
	// from-ID) so forensic queries against the audit log can answer
	// "which specific entity did Alice try to mutate?" without
	// re-parsing the deny summary string. ToID is omitted because
	// RR-F9M9 removed it from RelationSubject.
	var subject *audit.Subject
	switch s := req.Subject.(type) {
	case acl.RelationSubject:
		subject = &audit.Subject{
			Kind:         "relation",
			RelationType: s.Type,
			FromID:       s.FromID,
		}
	case acl.EntitySubject:
		subject = &audit.Subject{
			Kind: "entity",
			Type: s.Type,
			ID:   s.ID,
		}
	}
	m.deps.Audit.Record(audit.Record{
		Time:        time.Now().UTC(),
		Op:          audit.OpDeniedWrite,
		Subject:     subject,
		Principal:   principal.From(ctx),
		TriggeredBy: audit.TriggeredByFrom(ctx),
		Summary:     formatDeniedSummary(d, req.Op),
	})
}

// formatDeniedSummary builds the audit Summary for a denied-write row.
// Appends `attribution=[role=X via source, ...]` when the Decision
// carries Attributions so operators can answer "which roles did the
// resolver consider and via which paths" without re-running the
// resolver (AC7). The wire 403 path stays opaque — only audit reads
// Attributions.
func formatDeniedSummary(d acl.Decision, op acl.Op) string {
	base := fmt.Sprintf("denied: %s (rule_kind=%s rule_id=%s) attempted op=%s",
		d.Reason, d.RuleKind, d.RuleID, op)
	if len(d.Attributions) == 0 {
		return base
	}
	parts := make([]string, 0, len(d.Attributions))
	for _, a := range d.Attributions {
		parts = append(parts, fmt.Sprintf("role=%s via %s", a.Role, a.Source.String()))
	}
	return base + " attribution=[" + strings.Join(parts, ", ") + "]"
}

// CreateEntity creates a new entity, runs on-create automations, and
// dispatches any resulting cascade.
//
// Pipeline:
//
//  1. createCore: ID generation, template application, defaults,
//     metamodel validation, persist to store.
//  2. If automation should run: engine.Process(EventEntityCreated) →
//     collect property changes → apply → re-persist (yes, two writes:
//     the first is the validated bare entity, the second carries any
//     automation-set properties; pinned by manager_test.go).
//  3. Dispatch cascade via Cascade.Process; merge outcome into the
//     entity.CreateResult.
//
// **Caller-entity mutation.** The supplied `*entity.Entity` is used as
// a property/content carrier and not retained — the freshly-built
// entity is returned via [entity.CreateResult.Entity]. Callers should consume
// the returned entity, not the one they passed in.
func (m *Manager) CreateEntity(
	ctx context.Context, e *entity.Entity, opts entity.CreateOptions,
) (*entity.CreateResult, error) {
	if e == nil {
		return nil, errors.New("entitymanager: CreateEntity: entity is nil")
	}
	if err := m.authorizeAndAudit(ctx, acl.WriteRequest{
		Op:      acl.OpCreate,
		Subject: acl.EntitySubject{Type: e.Type, ID: opts.ID},
	}); err != nil {
		return nil, err
	}
	if opts.ID != "" {
		if def, ok := m.deps.Meta.GetEntityDef(e.Type); ok && !def.IsManualID() {
			return nil, customIDNotAllowedError(e.Type, def, opts.ID)
		}
		// No GetEntity pre-check: it was a TOCTOU duplicate of the
		// store's atomic uniqueness guarantee. createCore now writes
		// with a direct CreateEntity and surfaces a conflict as
		// ErrEntityAlreadyExists, so a racing create can't slip past a
		// passed pre-check and become an overwrite.
	}

	created, warnings, err := createCore(ctx, m.deps, e.Type, createCoreOpts{
		ID:              opts.ID,
		IDPrefix:        opts.Prefix,
		TemplateVariant: opts.Variant,
		Properties:      e.Properties,
		Content:         e.Content,
	})
	if err != nil {
		return nil, err
	}

	// Enforce state-machine entry: a create may only enter a machine at its
	// entry value (Initial, else Default). Runs post-template (defaults have
	// applied) on the created state (TKT-E4LW2). createCore persisted the row,
	// so a rejection here leaves an orphan on disk only briefly — but the
	// common path is a legal entry; an illegal explicit entry value is a client
	// error surfaced as 422 before automation/cascade run.
	if err := m.deps.Transitions.EnforceCreate(ctx, created); err != nil {
		return nil, err
	}

	result := &entity.CreateResult{Entity: created, Warnings: warnings}

	// Audit the durable write now, before automation re-writes or the
	// cascade run. createCore has already persisted the entity; if a
	// later step fails (the post-automation upsert, or a cascade that
	// hard-errors) the entity is still on disk, so the audit log must
	// already reflect it. Recording after the cascade left a window
	// where a committed write produced no audit record.
	m.recordEntityAudit(ctx, audit.OpCreateEntity, created, "created")

	runAutomation := m.deps.Automations != nil && !opts.SkipAutomation
	if !runAutomation {
		return result, nil
	}

	autoResult := m.deps.Automations.Process(automation.Event{
		Type:   automation.EventEntityCreated,
		Entity: created,
	})
	if len(autoResult.PropertiesSet) > 0 {
		for prop, val := range autoResult.PropertiesSet {
			created.SetString(prop, val)
		}
		// Re-enforce unique constraints against the POST-automation values:
		// createCore's check ran before automations, so an automation that
		// set a `unique` property could otherwise write a duplicate natural
		// key that the update path would reject (the create path must not be
		// the weaker one). excludeSelfID is created.ID — the entity is
		// already persisted from createCore, so it must not collide with
		// itself. A violation aborts before the duplicate is re-written.
		if err := checkUniqueProperties(ctx, m.deps, created, created.ID); err != nil {
			return nil, err
		}
		// UpdateEntity, not upsert: createCore already persisted this row
		// above, so the post-automation re-write is unambiguously an
		// update of an existing entity (BUG-ZWTDH9 — no create-then-
		// update fallback anywhere).
		if writeErr := m.deps.Store.UpdateEntity(ctx, created); writeErr != nil {
			return nil, fmt.Errorf("write entity after automation: %w", writeErr)
		}
		// Recompute warnings against the post-automation state
		// (DEC-HWZHA). The pre-write warnings from createCore reflect
		// the entity before automation set any properties.
		if errs := m.deps.Meta.ValidateEntity(created.ID, created.Type, created.Properties); len(errs) > 0 {
			_, result.Warnings = partitionValidationErrors(errs)
		}
	}
	result.AutomationWarnings = autoResult.Warnings
	result.AutomationErrors = autoResult.Errors

	outcome, cascadeErr := m.deps.Cascade.Process(ctx, &cascadeHost{deps: m.deps}, autocascade.Request{
		Trigger:    created,
		OldTrigger: nil,
		Result:     autoResult,
		Scripts:    m.deps.ScriptRunner,
		Mutator:    m.gated(), // gated() so elevation never propagates into a nested cascade (TKT-D8T148)
	})
	if cascadeErr != nil {
		return nil, fmt.Errorf("cascade: %w", cascadeErr)
	}
	result.RelationsCreated = outcome.RelationsCreated
	result.EntitiesCreated = outcome.EntitiesCreated
	result.AutomationErrors = append(result.AutomationErrors, outcome.Errors...)
	result.AutomationWarnings = append(result.AutomationWarnings, outcome.Warnings...)

	return result, nil
}

// ValidateCreate runs the create path's defaults + validation against a
// candidate entity WITHOUT persisting, authorizing, auditing, or
// running automation. It returns the would-be entity (post template /
// status defaults) and the DEC-HWZHA soft warnings the real create
// would surface — so a dry-run create can show as-you-type validation
// feedback that cannot drift from [Manager.CreateEntity] (both share
// [buildCandidateEntity]).
//
// Contract:
//   - No write, no audit row, no automation — it is advisory only.
//     The real CreateEntity remains the sole authorization and audit
//     point; callers MUST re-authorize at commit (e.g. the data-entry
//     create handler's affordance gate).
//   - Hard structural errors (unknown type, bad manual ID, ID-prefix
//     mismatch) return as an error; soft conditions (required-unset,
//     type / value mismatch) return as warnings on a nil error.
//   - opts.ID may be empty: an ID is generated only to satisfy
//     validation that doesn't depend on it; it is not reserved.
func (m *Manager) ValidateCreate(
	ctx context.Context, e *entity.Entity, opts entity.CreateOptions,
) (*entity.Entity, []entity.Warning, error) {
	if e == nil {
		return nil, nil, errors.New("entitymanager: ValidateCreate: entity is nil")
	}
	return buildCandidateEntity(ctx, m.deps, e.Type, createCoreOpts{
		ID:              opts.ID,
		IDPrefix:        opts.Prefix,
		TemplateVariant: opts.Variant,
		Properties:      e.Properties,
		Content:         e.Content,
		// Skip the full-store scan generateID would do — dry-run runs
		// per debounced keystroke and a real ID is not needed for
		// validation. RR-8I07.
		SkipIDGeneration: true,
	})
}

// UpdateEntity validates the new state, runs on-update automation
// when an old state is available, applies property changes, persists,
// and dispatches the cascade.
//
// **Caller-entity mutation.** When automation sets properties via
// [automation.Result.PropertiesSet], UpdateEntity mutates the supplied
// `*entity.Entity` in place before writing. Callers that need to
// preserve the pre-call state should clone first.
//
// **Gate:** if the entity doesn't exist, UpdateEntity returns
// [ErrEntityNotFound] and never runs the engine. (Preserves
// pre-refactor workspace behavior.)
func (m *Manager) UpdateEntity(ctx context.Context, e *entity.Entity) (*entity.UpdateResult, error) {
	if e == nil {
		return nil, errors.New("entitymanager: UpdateEntity: entity is nil")
	}
	if err := m.authorizeAndAudit(ctx, acl.WriteRequest{
		Op:      acl.OpUpdate,
		Subject: acl.EntitySubject{Type: e.Type, ID: e.ID},
	}); err != nil {
		return nil, err
	}
	// DEC-HWZHA: partition validation errors once. Hard errors abort;
	// soft conditions populate Result.Warnings. If automation runs and
	// mutates properties, we recompute warnings against the post-
	// automation state.
	preErrs := m.deps.Meta.ValidateEntity(e.ID, e.Type, e.Properties)
	hard, soft := partitionValidationErrors(preErrs)
	if len(hard) > 0 {
		return nil, newValidationError(hard)
	}

	oldEntity, getErr := m.deps.Store.GetEntity(ctx, e.ID)
	if getErr != nil {
		return nil, fmt.Errorf("%w: %s", ErrEntityNotFound, e.ID)
	}

	result := &entity.UpdateResult{Entity: e, Warnings: soft}

	runAutomation := m.deps.Automations != nil
	var autoResult *automation.Result
	if runAutomation {
		autoResult = m.deps.Automations.Process(automation.Event{
			Type:      automation.EventEntityUpdated,
			Entity:    e,
			OldEntity: oldEntity,
		})
		if len(autoResult.PropertiesSet) > 0 {
			for prop, val := range autoResult.PropertiesSet {
				e.SetString(prop, val)
			}
			// Properties changed — recompute warnings against the
			// post-automation state (DEC-HWZHA).
			if errs := m.deps.Meta.ValidateEntity(e.ID, e.Type, e.Properties); len(errs) > 0 {
				_, result.Warnings = partitionValidationErrors(errs)
			} else {
				result.Warnings = nil
			}
		}
		result.AutomationWarnings = autoResult.Warnings
		result.AutomationErrors = autoResult.Errors
	}

	// Enforce enum state machines on the final (post-automation) state, using
	// the prior state to determine the transition (TKT-E4LW2). This is the
	// unforgettable chokepoint: the enforcer is a required collaborator run in
	// the fixed write pipeline, so no update path can skip legality/guard/
	// precondition. An empty enforcer (metamodel with no transitions) is a
	// no-op.
	if err := m.deps.Transitions.EnforceUpdate(
		ctx, oldEntity, e, m.deps.TransitionGuard, m.deps.TransitionGraph,
	); err != nil {
		return nil, m.mapTransitionError(ctx, acl.EntitySubject{Type: e.Type, ID: e.ID}, err)
	}

	// Enforce `unique: true` natural-key constraints against the final
	// (post-automation) property values, excluding this entity's own
	// prior version so a re-save of an unchanged value does not collide.
	if err := checkUniqueProperties(ctx, m.deps, e, e.ID); err != nil {
		return nil, err
	}

	// UpdateEntity, not upsert: the GetEntity above already established
	// the row exists (else we returned ErrEntityNotFound), so this is
	// unambiguously an update (BUG-ZWTDH9).
	if err := m.deps.Store.UpdateEntity(ctx, e); err != nil {
		return nil, fmt.Errorf("write entity: %w", err)
	}

	// Audit the durable write now, before the cascade run. The entity
	// is already persisted; gating the audit on cascade success left a
	// window where a committed write produced no audit record.
	m.recordEntityAudit(ctx, audit.OpUpdateEntity, e, updateEntitySummary(oldEntity, e))

	if !runAutomation {
		return result, nil
	}

	outcome, cascadeErr := m.deps.Cascade.Process(ctx, &cascadeHost{deps: m.deps}, autocascade.Request{
		Trigger:    e,
		OldTrigger: oldEntity,
		Result:     autoResult,
		Scripts:    m.deps.ScriptRunner,
		Mutator:    m.gated(), // gated() so elevation never propagates into a nested cascade (TKT-D8T148)
	})
	if cascadeErr != nil {
		return nil, fmt.Errorf("cascade: %w", cascadeErr)
	}
	result.RelationsCreated = outcome.RelationsCreated
	result.EntitiesCreated = outcome.EntitiesCreated
	result.AutomationErrors = append(result.AutomationErrors, outcome.Errors...)
	result.AutomationWarnings = append(result.AutomationWarnings, outcome.Warnings...)

	return result, nil
}

// DeleteEntity removes an entity and its incident relations.
// **No automation, no cascade.** When cascade is false and the
// entity has any incident relations, returns [ErrHasRelations]
// without deleting anything.
func (m *Manager) DeleteEntity(ctx context.Context, id string, cascade bool) (*entity.DeleteResult, error) {
	current, err := m.deps.Store.GetEntity(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", ErrEntityNotFound, id)
	}
	// ACL check happens after the lookup so the request carries the
	// real entity type; a deny on a non-existent entity would be more
	// confusing than the ErrEntityNotFound returned above.
	if aclErr := m.authorizeAndAudit(ctx, acl.WriteRequest{
		Op:      acl.OpDelete,
		Subject: acl.EntitySubject{Type: current.Type, ID: id},
	}); aclErr != nil {
		return nil, aclErr
	}

	incoming, err := collectIncidentRelations(ctx, m.deps.Store, id, store.DirectionIncoming)
	if err != nil {
		return nil, fmt.Errorf("collect incoming relations for %q: %w", id, err)
	}
	outgoing, err := collectIncidentRelations(ctx, m.deps.Store, id, store.DirectionOutgoing)
	if err != nil {
		return nil, fmt.Errorf("collect outgoing relations for %q: %w", id, err)
	}
	totalRelations := len(incoming) + len(outgoing)

	if totalRelations > 0 && !cascade {
		return nil, ErrHasRelations
	}

	// Capture the final pre-delete version BEFORE the store delete. The row is
	// hard-deleted below, so a version taken after the delete would be
	// unrecoverable if the process died in between — order-before closes that
	// permanent-loss window (the store delete is still not transactional with
	// this capture; strict atomicity is a future hardening).
	m.recordEntityVersion(ctx, store.VersionOpDelete, current, "")

	// Capture a final version for every relation this cascade destroys, BEFORE
	// the store delete removes them. This is the ONLY place cascade-deleted
	// relations get versioned: the store's DeleteEntity bulk-deletes them below
	// the write choke-point, so without this their history would silently end
	// with no `delete` marker and no restore path (RR-181AFY). Each is attributed
	// to the triggering entity delete. Live rows still exist here, so
	// WriteRelationVersion resolves rel_record_id from the key.
	if cascade && totalRelations > 0 {
		cascadeTB := "cascade:delete-entity:" + id
		for _, rel := range incoming {
			m.recordRelationVersion(ctx, store.VersionOpDelete, rel, "", "", cascadeTB)
		}
		for _, rel := range outgoing {
			m.recordRelationVersion(ctx, store.VersionOpDelete, rel, "", "", cascadeTB)
		}
	}

	// Delegate the actual deletion to the store's cascade, which removes
	// the relation files and the entity file under a single lock and aborts
	// fail-secure if any relation file cannot be removed — so the entity is
	// never deleted while a relation is left behind (issue #888). A real
	// error surfaces to the caller rather than being swallowed; previously
	// the Manager looped per-relation and `continue`d past I/O failures,
	// deleting the entity anyway and leaving orphans untraced.
	res, delErr := m.deps.Store.DeleteEntity(ctx, id, cascade)
	if delErr != nil {
		return nil, fmt.Errorf("delete entity: %w", delErr)
	}

	// Audit exactly what the store reports deleting. Cascade-deleted
	// relations carry triggered_by so the log attributes them to this
	// delete; recordRelationAudit reads it from cascadeCtx.
	cascadeCtx := ctx
	if cascade && len(res.DeletedRelations) > 0 {
		cascadeCtx = audit.WithTriggeredBy(ctx, "cascade:delete-entity:"+id)
	}
	for _, rel := range res.DeletedRelations {
		m.recordRelationAudit(cascadeCtx, audit.OpDeleteRelation, rel, "deleted")
	}

	deleteSummary := "deleted"
	if cascade && len(res.DeletedRelations) > 0 {
		deleteSummary = fmt.Sprintf("deleted (cascade: %d relations)", len(res.DeletedRelations))
	}
	m.recordEntityAudit(ctx, audit.OpDeleteEntity, current, deleteSummary)

	return &entity.DeleteResult{
		DeletedEntities:  []*entity.Entity{current},
		DeletedRelations: res.DeletedRelations,
	}, nil
}

// RenameEntity changes an entity's ID and rewrites all incident
// relations. **No automation, no cascade, no metamodel re-validation
// of the post-rename state** (preserved verbatim from pre-refactor
// workspace behavior).
//
// If opts.DryRun is true, no changes are persisted (and no audit
// record is emitted — dry runs do not show up in the audit log).
func (m *Manager) RenameEntity(
	ctx context.Context, oldID, newID string, opts entity.RenameOptions,
) (*entity.RenameResult, error) {
	// ACL needs the entity type, so we fetch first. Distinguish the two
	// failure modes:
	//   - not-found: skip ACL and fall through; renameEntity below
	//     returns ErrEntityNotFound with a clearer message, and there is
	//     nothing to authorize against.
	//   - any other error (transient I/O, backend hiccup): fail closed.
	//     Proceeding would run the rename with NO authorization at all —
	//     a store read that flakes must not turn an ACL-gated operation
	//     into an ungated one.
	current, getErr := m.deps.Store.GetEntity(ctx, oldID)
	switch {
	case getErr == nil:
		if aclErr := m.authorizeAndAudit(ctx, acl.WriteRequest{
			Op:      acl.OpRename,
			Subject: acl.EntitySubject{Type: current.Type, ID: oldID},
		}); aclErr != nil {
			return nil, aclErr
		}
	case !errors.Is(getErr, store.ErrNotFound):
		return nil, fmt.Errorf("rename: load entity %q: %w", oldID, getErr)
	}

	// Collect incident relations (with their content) BEFORE the rename: the
	// rename rewrites each relation as create-new-triple + delete-old-triple at
	// the store level, so afterward the old endpoints are gone. We capture the
	// pre-rename state here to emit a `rename` version per relation below, so a
	// renamed endpoint's relation history stays continuous instead of reading as
	// a mass delete+create.
	var preRenameRels []*entity.Relation
	if !opts.DryRun && m.deps.RelationVersionRecorder != nil {
		preRenameRels = collectRenameAffectedRelations(ctx, m.deps.Store, oldID)
	}

	res, err := renameEntity(ctx, m.deps.Store, oldID, newID, opts)
	if err != nil || opts.DryRun {
		return res, err
	}

	// Derive both before/after subjects from the post-rename entity:
	// type is preserved by rename, so the post entity has the type
	// for both records. A separate pre-fetch would create a window
	// where audit silently no-ops if the pre-fetch fails but
	// rename succeeds (concurrent insert / racy store).
	postEntity, getErr := m.deps.Store.GetEntity(ctx, newID)
	if getErr != nil {
		slog.Error("audit.write_failed",
			"stage", "rename-postfetch",
			"new_id", newID,
			"error", getErr)
		return res, nil
	}
	m.recordRenameAudit(ctx, oldID, postEntity)
	// Capture the rename as a version event carrying the old id (prev_id), so a
	// renamed entity's history is walkable back to its former id. Only the
	// choke-point knows old->new; a later sweep sees the renamed entity as an
	// ordinary update and cannot reconstruct this link.
	m.recordEntityVersion(ctx, store.VersionOpRename, postEntity, oldID)

	// Capture a `rename` version for each incident relation, on its NEW triple,
	// carrying the pre-rename endpoints (prev_from/prev_to). This stitches the
	// relation's history across the endpoint rename so it reads as one continuous
	// timeline rather than a delete of the old triple + create of the new one.
	// The new triples exist now (the store's atomic RenameEntity created them);
	// the version's key is the post-rename endpoint, so WriteRelationVersion
	// resolves the new rel_record_id. triggered_by attributes the versions to
	// this rename.
	if len(preRenameRels) > 0 {
		renameTB := "rename-entity:" + oldID + "->" + newID
		for _, rel := range preRenameRels {
			newFrom, newTo := rel.From, rel.To
			if newFrom == oldID {
				newFrom = newID
			}
			if newTo == oldID {
				newTo = newID
			}
			after := &entity.Relation{
				From: newFrom, Type: rel.Type, To: newTo,
				Properties: rel.Properties, Content: rel.Content,
			}
			m.recordRelationVersion(ctx, store.VersionOpRename, after, rel.From, rel.To, renameTB)
		}
	}
	return res, nil
}

// collectRenameAffectedRelations gathers the incident relations of id (both
// directions) with their content, for pre-rename version capture. Self-
// referential relations appear once (outgoing). Errors are swallowed — a
// best-effort capture must never fail the rename.
func collectRenameAffectedRelations(ctx context.Context, st store.Store, id string) []*entity.Relation {
	seen := make(map[string]struct{})
	out := make([]*entity.Relation, 0)
	for _, dir := range []store.Direction{store.DirectionOutgoing, store.DirectionIncoming} {
		for r, err := range st.ListRelations(ctx, store.RelationQuery{EntityID: id, Direction: dir}) {
			if err != nil {
				continue
			}
			key := r.From + "\x00" + r.Type + "\x00" + r.To
			if _, dup := seen[key]; dup {
				continue
			}
			seen[key] = struct{}{}
			out = append(out, r)
		}
	}
	return out
}

// CreateRelation creates a new relation, validating endpoints and
// the relation-type tuple against the metamodel. **No automation.**
func (m *Manager) CreateRelation(
	ctx context.Context, from, relType, to string, opts entity.RelationOptions,
) (*entity.Relation, error) {
	// Authorize BEFORE the peer-existence lookups (BUG-K6FEVB). A missing
	// peer must never let a write skip the ACL: if authz is deferred until
	// after GetEntity, a denied caller (e.g. --read-only / ReadOnlyACL)
	// gets a soft "entity not found" instead of a *acl.ForbiddenError,
	// and the dataentry fallback then writes directly to the store,
	// bypassing the ACL and audit. The source type feeds the type-level
	// grant check; it is best-effort (empty if the source doesn't exist
	// yet), mirroring UpdateRelation/DeleteRelation. Authorization must be
	// decided from inputs that don't depend on peer existence.
	var fromType string
	if fromEntity, ferr := m.deps.Store.GetEntity(ctx, from); ferr == nil {
		fromType = fromEntity.Type
	}
	if aclErr := m.authorizeAndAudit(ctx, acl.WriteRequest{
		Op: acl.OpCreate,
		Subject: acl.RelationSubject{
			Type:     relType,
			FromType: fromType, FromID: from,
		},
	}); aclErr != nil {
		return nil, aclErr
	}

	fromEntity, err := m.deps.Store.GetEntity(ctx, from)
	if err != nil {
		return nil, fmt.Errorf("source %w: %s", ErrEntityNotFound, from)
	}
	toEntity, err := m.deps.Store.GetEntity(ctx, to)
	if err != nil {
		return nil, fmt.Errorf("target %w: %s", ErrEntityNotFound, to)
	}
	if vErr := m.deps.Meta.ValidateRelation(relType, fromEntity.Type, toEntity.Type); vErr != nil {
		return nil, fmt.Errorf("invalid relation: %w", vErr)
	}
	if _, gErr := m.deps.Store.GetRelation(ctx, from, relType, to); gErr == nil {
		return nil, fmt.Errorf("%w: %s --%s--> %s", ErrRelationAlreadyExists, from, relType, to)
	}

	rel := entity.NewRelation(from, relType, to)

	tmpl, err := m.deps.Templater.RelationTemplate(ctx, relType)
	if err != nil {
		return nil, fmt.Errorf("load relation template: %w", err)
	}
	if tmpl != nil {
		rel.Properties = templating.ApplyRelation(rel.Properties, tmpl)
	}

	if len(opts.Properties) > 0 && rel.Properties == nil {
		rel.Properties = make(map[string]interface{})
	}
	for k, v := range opts.Properties {
		rel.Properties[k] = v
	}
	if opts.Content != nil {
		rel.Content = *opts.Content
	}

	// Auto-assign managed order properties (_order_out / _order_in) when
	// the relation type declares the side orderable. Overrides any
	// non-finite caller-supplied value with AppendOrder over existing
	// siblings; keeps finite caller values as-is.
	if err := m.assignManagedOrder(ctx, rel, relType); err != nil {
		return nil, err
	}

	// CreateRelation, not upsert: a create must never fall through to an
	// update (that would clobber a racing create of the same triple).
	// The GetRelation pre-check above is advisory; the store's atomic
	// create is the real guard, and a conflict surfaces as
	// ErrRelationAlreadyExists (BUG-ZWTDH9).
	if _, err := m.deps.Store.CreateRelation(ctx, from, relType, to, &store.RelationData{
		Properties: rel.Properties,
		Content:    rel.Content,
	}); err != nil {
		if errors.Is(err, store.ErrConflict) {
			return nil, fmt.Errorf("%w: %s --%s--> %s", ErrRelationAlreadyExists, from, relType, to)
		}
		return nil, err
	}
	m.recordRelationAudit(ctx, audit.OpCreateRelation, rel, "created")
	return rel, nil
}

// UpdateRelation merges new properties into an existing relation,
// applies MetaUnset, optionally replaces content, and persists.
// **No automation, no metamodel re-validation.**
func (m *Manager) UpdateRelation(
	ctx context.Context, from, relType, to string, opts entity.RelationOptions,
) (*entity.Relation, error) {
	// Authorize BEFORE the relation-existence lookup (BUG-K6FEVB): a
	// missing relation must not let a denied caller skip the ACL and get
	// a soft not-found. The source type feeds the type-level grant check;
	// it is best-effort (empty if the source doesn't exist).
	var sourceType string
	if fromEntity, ferr := m.deps.Store.GetEntity(ctx, from); ferr == nil {
		sourceType = fromEntity.Type
	}
	if aclErr := m.authorizeAndAudit(ctx, acl.WriteRequest{
		Op: acl.OpUpdate,
		Subject: acl.RelationSubject{
			Type:     relType,
			FromType: sourceType, FromID: from,
		},
	}); aclErr != nil {
		return nil, aclErr
	}

	rel, err := m.deps.Store.GetRelation(ctx, from, relType, to)
	if err != nil {
		return nil, fmt.Errorf("%w: %s --%s--> %s", ErrRelationNotFound, from, relType, to)
	}

	// Snapshot pre-update meta keys so the audit summary names exactly
	// which keys changed (values never appear).
	oldProps := cloneProperties(rel.Properties)

	// Reject non-finite numeric values on managed order properties.
	// HTTP wire validators already cover the dataentry path; this is
	// the engine-level backstop for MCP/Lua/CLI write paths.
	relDef, hasDef := m.deps.Meta.Relations[relType]
	touchedOut := hasDef && relDef.OutgoingOrderProperty() != "" && touchesOrderKey(opts, relDef.OutgoingOrderProperty())
	touchedIn := hasDef && relDef.IncomingOrderProperty() != "" && touchesOrderKey(opts, relDef.IncomingOrderProperty())
	if hasDef {
		if err := validateOrderUpdate(opts, relDef); err != nil {
			return nil, err
		}
	}

	if rel.Properties == nil && (len(opts.Properties) > 0 || len(opts.MetaUnset) > 0) {
		rel.Properties = make(map[string]interface{})
	}
	for k, v := range opts.Properties {
		rel.Properties[k] = v
	}
	for _, k := range opts.MetaUnset {
		delete(rel.Properties, k)
	}
	if opts.Content != nil {
		rel.Content = *opts.Content
	}

	// UpdateRelation, not upsert: the GetRelation above established the
	// triple exists (else we returned ErrRelationNotFound), so this is
	// unambiguously an update (BUG-ZWTDH9).
	if _, err := m.deps.Store.UpdateRelation(ctx, from, relType, to, store.RelationData{
		Properties: rel.Properties,
		Content:    rel.Content,
	}); err != nil {
		return nil, err
	}
	m.recordRelationAudit(ctx, audit.OpUpdateRelation, rel, updateRelationSummary(oldProps, rel.Properties))

	// Engine-initiated renumber when an order PATCH collapsed sibling
	// spacing. Errors are operator-visible (slog.Error) but do not fail
	// the user-visible Update — the caller's write already succeeded.
	m.runRenumberAfterUpdate(ctx, from, to, relType, touchedOut, touchedIn)

	return rel, nil
}

// DeleteRelation removes a relation. **No automation.**
func (m *Manager) DeleteRelation(ctx context.Context, from, relType, to string) error {
	// Authorize BEFORE touching the store (BUG-K6FEVB). The source type
	// feeds the type-level grant check; it is best-effort (empty if the
	// source doesn't exist).
	var sourceType string
	if fromEntity, ferr := m.deps.Store.GetEntity(ctx, from); ferr == nil {
		sourceType = fromEntity.Type
	}
	if aclErr := m.authorizeAndAudit(ctx, acl.WriteRequest{
		Op: acl.OpDelete,
		Subject: acl.RelationSubject{
			Type:     relType,
			FromType: sourceType, FromID: from,
		},
	}); aclErr != nil {
		return aclErr
	}
	// Fetch pre-delete AFTER authz (BUG-K6FEVB: a denied delete must return
	// ForbiddenError regardless of whether the relation exists) so the audit
	// record and version snapshot carry the full Subject (relation type + from
	// + to). The relation may not exist — then the store delete returns an
	// error and we skip both the version capture and the audit.
	rel, getErr := m.deps.Store.GetRelation(ctx, from, relType, to)
	// Capture the final pre-delete version BEFORE the store delete, while the
	// live row (and its rel_record_id) still exists — the same order-before
	// rationale as entity delete. Skipped if the relation was already gone.
	if getErr == nil {
		m.recordRelationVersion(ctx, store.VersionOpDelete, rel, "", "", "")
	}
	if err := m.deps.Store.DeleteRelation(ctx, from, relType, to); err != nil {
		return fmt.Errorf("delete relation: %w", err)
	}
	if getErr == nil {
		m.recordRelationAudit(ctx, audit.OpDeleteRelation, rel, "deleted")
	}
	return nil
}
