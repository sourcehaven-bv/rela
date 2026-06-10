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
func (m *Manager) authorizeAndAudit(ctx context.Context, req acl.WriteRequest) error {
	decision := m.deps.ACL.AuthorizeWrite(ctx, req)
	if decision.Allow {
		return nil
	}
	m.recordDeniedWrite(ctx, decision, req)
	return &acl.ForbiddenError{Decision: decision}
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
		if _, err := m.deps.Store.GetEntity(ctx, opts.ID); err == nil {
			return nil, fmt.Errorf("%w: %s", ErrEntityAlreadyExists, opts.ID)
		}
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

	result := &entity.CreateResult{Entity: created, Warnings: warnings}

	runAutomation := m.deps.Automations != nil && !opts.SkipAutomation
	if !runAutomation {
		m.recordEntityAudit(ctx, audit.OpCreateEntity, created, "created")
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
		if writeErr := upsertEntity(ctx, m.deps.Store, created); writeErr != nil {
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
		Mutator:    m, // Manager satisfies autocascade.Mutator structurally
	})
	if cascadeErr != nil {
		return nil, fmt.Errorf("cascade: %w", cascadeErr)
	}
	result.RelationsCreated = outcome.RelationsCreated
	result.EntitiesCreated = outcome.EntitiesCreated
	result.AutomationErrors = append(result.AutomationErrors, outcome.Errors...)
	result.AutomationWarnings = append(result.AutomationWarnings, outcome.Warnings...)

	m.recordEntityAudit(ctx, audit.OpCreateEntity, created, "created")
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

	if err := upsertEntity(ctx, m.deps.Store, e); err != nil {
		return nil, fmt.Errorf("write entity: %w", err)
	}

	updateSummary := updateEntitySummary(oldEntity, e)

	if !runAutomation {
		m.recordEntityAudit(ctx, audit.OpUpdateEntity, e, updateSummary)
		return result, nil
	}

	outcome, cascadeErr := m.deps.Cascade.Process(ctx, &cascadeHost{deps: m.deps}, autocascade.Request{
		Trigger:    e,
		OldTrigger: oldEntity,
		Result:     autoResult,
		Scripts:    m.deps.ScriptRunner,
		Mutator:    m, // Manager satisfies autocascade.Mutator structurally
	})
	if cascadeErr != nil {
		return nil, fmt.Errorf("cascade: %w", cascadeErr)
	}
	result.RelationsCreated = outcome.RelationsCreated
	result.EntitiesCreated = outcome.EntitiesCreated
	result.AutomationErrors = append(result.AutomationErrors, outcome.Errors...)
	result.AutomationWarnings = append(result.AutomationWarnings, outcome.Warnings...)

	m.recordEntityAudit(ctx, audit.OpUpdateEntity, e, updateSummary)
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

	incoming := collectIncidentRelations(ctx, m.deps.Store, id, store.DirectionIncoming)
	outgoing := collectIncidentRelations(ctx, m.deps.Store, id, store.DirectionOutgoing)
	totalRelations := len(incoming) + len(outgoing)

	if totalRelations > 0 && !cascade {
		return nil, ErrHasRelations
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
	return res, nil
}

// CreateRelation creates a new relation, validating endpoints and
// the relation-type tuple against the metamodel. **No automation.**
func (m *Manager) CreateRelation(
	ctx context.Context, from, relType, to string, opts entity.RelationOptions,
) (*entity.Relation, error) {
	fromEntity, err := m.deps.Store.GetEntity(ctx, from)
	if err != nil {
		return nil, fmt.Errorf("source %w: %s", ErrEntityNotFound, from)
	}
	toEntity, err := m.deps.Store.GetEntity(ctx, to)
	if err != nil {
		return nil, fmt.Errorf("target %w: %s", ErrEntityNotFound, to)
	}
	if aclErr := m.authorizeAndAudit(ctx, acl.WriteRequest{
		Op: acl.OpCreate,
		Subject: acl.RelationSubject{
			Type:     relType,
			FromType: fromEntity.Type, FromID: from,
		},
	}); aclErr != nil {
		return nil, aclErr
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

	if err := upsertRelation(ctx, m.deps.Store, rel); err != nil {
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
	rel, err := m.deps.Store.GetRelation(ctx, from, relType, to)
	if err != nil {
		return nil, fmt.Errorf("%w: %s --%s--> %s", ErrRelationNotFound, from, relType, to)
	}
	// ACL needs the source entity type for the type-level write check.
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

	if err := upsertRelation(ctx, m.deps.Store, rel); err != nil {
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
	// Fetch pre-delete so the audit record carries the full Subject
	// (relation type + from + to). The relation may not exist — in
	// that case the store delete returns an error and we skip audit.
	rel, getErr := m.deps.Store.GetRelation(ctx, from, relType, to)
	// ACL needs the source entity type for the type-level write check.
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
	if err := m.deps.Store.DeleteRelation(ctx, from, relType, to); err != nil {
		return fmt.Errorf("delete relation: %w", err)
	}
	if getErr == nil {
		m.recordRelationAudit(ctx, audit.OpDeleteRelation, rel, "deleted")
	}
	return nil
}
