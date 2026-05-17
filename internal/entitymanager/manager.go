package entitymanager

import (
	"context"
	"errors"
	"fmt"

	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/autocascade"
	"github.com/Sourcehaven-BV/rela/internal/automation"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
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
	if (d.Automations == nil) != (d.Cascade == nil) {
		return nil, errors.New(
			"entitymanager: New: Automations and Cascade must be supplied together (both non-nil or both nil)",
		)
	}
	return &Manager{deps: d}, nil
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

	incoming := collectIncidentRelations(ctx, m.deps.Store, id, store.DirectionIncoming)
	outgoing := collectIncidentRelations(ctx, m.deps.Store, id, store.DirectionOutgoing)
	totalRelations := len(incoming) + len(outgoing)

	if totalRelations > 0 && !cascade {
		return nil, ErrHasRelations
	}

	// Cascade-deleted relations get triggered_by set so the audit log
	// records the parent op that caused them. Wrap ctx once before the
	// delete loop — recordRelationAudit reads triggered_by from this
	// derived ctx, the original ctx still flows to anything else that
	// needs the un-decorated principal context.
	cascadeCtx := ctx
	if cascade && totalRelations > 0 {
		cascadeCtx = audit.WithTriggeredBy(ctx, "cascade:delete-entity:"+id)
	}

	deletedRelations := make([]*entity.Relation, 0, totalRelations)
	for _, rel := range incoming {
		if delErr := m.deps.Store.DeleteRelation(ctx, rel.From, rel.Type, rel.To); delErr != nil &&
			!errors.Is(delErr, store.ErrNotFound) {

			continue
		}
		deletedRelations = append(deletedRelations, rel)
		m.recordRelationAudit(cascadeCtx, audit.OpDeleteRelation, rel, "deleted")
	}
	for _, rel := range outgoing {
		if delErr := m.deps.Store.DeleteRelation(ctx, rel.From, rel.Type, rel.To); delErr != nil &&
			!errors.Is(delErr, store.ErrNotFound) {

			continue
		}
		deletedRelations = append(deletedRelations, rel)
		m.recordRelationAudit(cascadeCtx, audit.OpDeleteRelation, rel, "deleted")
	}

	if _, delErr := m.deps.Store.DeleteEntity(ctx, id, false); delErr != nil &&
		!errors.Is(delErr, store.ErrNotFound) {

		return nil, fmt.Errorf("delete entity: %w", delErr)
	}

	deleteSummary := "deleted"
	if cascade && totalRelations > 0 {
		deleteSummary = fmt.Sprintf("deleted (cascade: %d relations)", totalRelations)
	}
	m.recordEntityAudit(ctx, audit.OpDeleteEntity, current, deleteSummary)

	return &entity.DeleteResult{
		DeletedEntities:  []*entity.Entity{current},
		DeletedRelations: deletedRelations,
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
	// Fetch pre-rename state so the audit record can include the old
	// Type. Avoids a second store lookup post-rename which would race
	// against concurrent deletes / re-renames.
	preEntity, getErr := m.deps.Store.GetEntity(ctx, oldID)
	if getErr != nil {
		// Pass through — renameEntity below will translate this into
		// the entitymanager.ErrEntityNotFound sentinel.
		preEntity = nil
	}

	res, err := renameEntity(ctx, m.deps.Store, oldID, newID, opts)
	if err != nil || opts.DryRun {
		return res, err
	}

	postEntity, _ := m.deps.Store.GetEntity(ctx, newID)
	m.recordRenameAudit(ctx, preEntity, postEntity)
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

	// Snapshot pre-update meta keys so the audit summary names exactly
	// which keys changed (values never appear).
	oldProps := cloneProperties(rel.Properties)

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
	return rel, nil
}

// DeleteRelation removes a relation. **No automation.**
func (m *Manager) DeleteRelation(ctx context.Context, from, relType, to string) error {
	// Fetch pre-delete so the audit record carries the full Subject
	// (relation type + from + to). The relation may not exist — in
	// that case the store delete returns an error and we skip audit.
	rel, getErr := m.deps.Store.GetRelation(ctx, from, relType, to)
	if err := m.deps.Store.DeleteRelation(ctx, from, relType, to); err != nil {
		return fmt.Errorf("delete relation: %w", err)
	}
	if getErr == nil {
		m.recordRelationAudit(ctx, audit.OpDeleteRelation, rel, "deleted")
	}
	return nil
}
