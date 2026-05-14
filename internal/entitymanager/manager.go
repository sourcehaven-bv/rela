package entitymanager

import (
	"context"
	"errors"
	"fmt"

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
	// transport-specific deps (e.g. Lua) supply a per-request adapter
	// here (see internal/workspace/luascriptrunner.go).
	ScriptRunner autocascade.ScriptRunner
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
//     CreateResult.
func (m *Manager) CreateEntity(ctx context.Context, e *entity.Entity, opts CreateOptions) (*CreateResult, error) {
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

	created, err := createCore(ctx, m.deps, e.Type, createCoreOpts{
		ID:              opts.ID,
		IDPrefix:        opts.Prefix,
		TemplateVariant: opts.Variant,
		Properties:      e.Properties,
		Content:         e.Content,
	})
	if err != nil {
		return nil, err
	}

	result := &CreateResult{Entity: created}

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
		if writeErr := upsertEntity(ctx, m.deps.Store, created); writeErr != nil {
			return nil, fmt.Errorf("write entity after automation: %w", writeErr)
		}
	}
	result.AutomationWarnings = autoResult.Warnings
	result.AutomationErrors = autoResult.Errors

	outcome, cascadeErr := m.deps.Cascade.Process(ctx, &cascadeHost{deps: m.deps}, autocascade.Request{
		Trigger:    created,
		OldTrigger: nil,
		Result:     autoResult,
		Scripts:    m.deps.ScriptRunner,
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

// UpdateEntity validates the new state, runs on-update automation
// when an old state is available, applies property changes, persists,
// and dispatches the cascade.
//
// **Gate:** if the entity doesn't exist, UpdateEntity returns
// [ErrEntityNotFound] and never runs the engine. (Preserves
// pre-refactor workspace behavior.)
func (m *Manager) UpdateEntity(ctx context.Context, e *entity.Entity) (*UpdateResult, error) {
	if e == nil {
		return nil, errors.New("entitymanager: UpdateEntity: entity is nil")
	}
	if errs := m.deps.Meta.ValidateEntity(e.ID, e.Type, e.Properties); len(errs) > 0 {
		return nil, newValidationError(errs)
	}

	oldEntity, getErr := m.deps.Store.GetEntity(ctx, e.ID)
	if getErr != nil {
		return nil, fmt.Errorf("%w: %s", ErrEntityNotFound, e.ID)
	}

	result := &UpdateResult{Entity: e}

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
		}
		result.AutomationWarnings = autoResult.Warnings
		result.AutomationErrors = autoResult.Errors
	}

	if err := upsertEntity(ctx, m.deps.Store, e); err != nil {
		return nil, fmt.Errorf("write entity: %w", err)
	}

	if !runAutomation {
		return result, nil
	}

	outcome, cascadeErr := m.deps.Cascade.Process(ctx, &cascadeHost{deps: m.deps}, autocascade.Request{
		Trigger:    e,
		OldTrigger: oldEntity,
		Result:     autoResult,
		Scripts:    m.deps.ScriptRunner,
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
func (m *Manager) DeleteEntity(ctx context.Context, id string, cascade bool) (*DeleteResult, error) {
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

	deletedRelations := make([]*entity.Relation, 0, totalRelations)
	for _, rel := range incoming {
		if delErr := m.deps.Store.DeleteRelation(ctx, rel.From, rel.Type, rel.To); delErr != nil &&
			!errors.Is(delErr, store.ErrNotFound) {

			continue
		}
		deletedRelations = append(deletedRelations, rel)
	}
	for _, rel := range outgoing {
		if delErr := m.deps.Store.DeleteRelation(ctx, rel.From, rel.Type, rel.To); delErr != nil &&
			!errors.Is(delErr, store.ErrNotFound) {

			continue
		}
		deletedRelations = append(deletedRelations, rel)
	}

	if _, delErr := m.deps.Store.DeleteEntity(ctx, id, false); delErr != nil &&
		!errors.Is(delErr, store.ErrNotFound) {

		return nil, fmt.Errorf("delete entity: %w", delErr)
	}

	return &DeleteResult{
		DeletedEntities:  []*entity.Entity{current},
		DeletedRelations: deletedRelations,
	}, nil
}

// RenameEntity changes an entity's ID and rewrites all incident
// relations. **No automation, no cascade, no metamodel re-validation
// of the post-rename state** (preserved verbatim from pre-refactor
// workspace behavior).
//
// If opts.DryRun is true, no changes are persisted.
func (m *Manager) RenameEntity(ctx context.Context, oldID, newID string, opts RenameOptions) (*RenameResult, error) {
	return renameEntity(ctx, m.deps.Store, oldID, newID, opts)
}

// CreateRelation creates a new relation, validating endpoints and
// the relation-type tuple against the metamodel. **No automation.**
func (m *Manager) CreateRelation(
	ctx context.Context, from, relType, to string, opts RelationOptions,
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
	return rel, nil
}

// UpdateRelation merges new properties into an existing relation,
// applies MetaUnset, optionally replaces content, and persists.
// **No automation, no metamodel re-validation.**
func (m *Manager) UpdateRelation(
	ctx context.Context, from, relType, to string, opts RelationOptions,
) (*entity.Relation, error) {
	rel, err := m.deps.Store.GetRelation(ctx, from, relType, to)
	if err != nil {
		return nil, fmt.Errorf("%w: %s --%s--> %s", ErrRelationNotFound, from, relType, to)
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
	return rel, nil
}

// DeleteRelation removes a relation. **No automation.**
func (m *Manager) DeleteRelation(ctx context.Context, from, relType, to string) error {
	if err := m.deps.Store.DeleteRelation(ctx, from, relType, to); err != nil {
		return fmt.Errorf("delete relation: %w", err)
	}
	return nil
}
