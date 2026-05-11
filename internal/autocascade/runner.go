package autocascade

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/Sourcehaven-BV/rela/internal/automation"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/lua"
)

// Runner executes the side effects of an automation cascade.
//
// Runner is constructed once (typically per Workspace or
// EntityManager) and invoked many times via [Runner.Process]. The
// caller supplies a [Host] on each call; that decision is what
// breaks the construction cycle between Runner and EntityManager
// (which holds Runner and satisfies Host).
type Runner struct {
	engine  *automation.Engine
	scripts Executor
}

// Deps is the constructor input for [New]. Using a struct keeps the
// constructor signature stable as Runner gains required collaborators
// (e.g. audit hooks via TKT-6YYM).
type Deps struct {
	// Engine is the rule-evaluation engine. Runner calls
	// engine.Process on each newly created entity in a cascade.
	Engine *automation.Engine

	// Scripts is the script executor used for automation Lua
	// actions. In production this is *script.Engine (which
	// satisfies Executor structurally); in tests it can be a stub
	// or [NopExecutor].
	Scripts Executor
}

// New constructs a Runner. Required collaborators (Engine, Scripts)
// must be non-nil per the project's "constructors reject nil required
// fields" rule.
func New(d Deps) (*Runner, error) {
	if d.Engine == nil {
		return nil, errors.New("autocascade: New: Engine is required")
	}
	if d.Scripts == nil {
		return nil, errors.New("autocascade: New: Scripts is required")
	}
	return &Runner{engine: d.Engine, scripts: d.Scripts}, nil
}

// queueItem is one pending automation result to process during a BFS
// cascade. Internal — callers never construct these.
type queueItem struct {
	trigger    *entity.Entity
	autoResult *automation.Result
}

// Process runs the BFS automation cascade. It interprets req.Result's
// actions (set properties, create relations, create entities, run
// Lua scripts), calling back into host for the structural operations
// and into the script executor for the Lua actions. Newly created
// entities are re-evaluated through the engine to discover further
// cascades, bounded by [MaxDepth].
//
// Behavior is preserved verbatim from
// workspace.applyAutomationSideEffects: the BFS order, the
// per-iteration action order (Lua → relations → entities), the
// error-continuation semantics across all action paths, and the
// depth-limit warning wording. The existing workspace cascade tests
// (AC3 in PLAN-V6UR) act as the regression check.
func (r *Runner) Process(ctx context.Context, host Host, req Request) (Outcome, error) {
	if req.Result == nil {
		return Outcome{}, nil
	}
	if req.Trigger == nil {
		return Outcome{}, errors.New("autocascade: Process: req.Trigger is required")
	}

	var outcome Outcome

	// BFS queue of pending automation results to process.
	queue := []queueItem{{req.Trigger, req.Result}}
	iterations := 0

	for len(queue) > 0 && iterations < MaxDepth {
		// Pop from front (BFS order — process all items at depth N
		// before depth N+1).
		item := queue[0]
		queue = queue[1:]
		iterations++

		// Process Lua scripts for this trigger.
		//
		// Note: req.OldTrigger is reused for every queue item, not
		// just the initial one. This mirrors
		// workspace.applyAutomationSideEffects (line 1067) — for
		// cascaded entities the original trigger's old state flows
		// through. Preserving the behavior; not "fixing" it here.
		r.executeLuaActions(item.trigger, req.OldTrigger, item.autoResult.LuaToExecute, req.LuaDeps, &outcome)

		// Process relations for this trigger.
		r.applyRelationCreations(ctx, host, item.trigger, item.autoResult.RelationsToCreate, &outcome)

		// Collect warnings/errors from this automation result.
		outcome.Warnings = append(outcome.Warnings, item.autoResult.Warnings...)
		outcome.Errors = append(outcome.Errors, item.autoResult.Errors...)

		// Process entity creations and collect any new queue items.
		newItems := r.processEntityCreations(ctx, host, item.trigger, item.autoResult.EntitiesToCreate, &outcome)
		queue = append(queue, newItems...)
	}

	// Warn if we hit the limit with work remaining.
	if len(queue) > 0 {
		outcome.Warnings = append(outcome.Warnings,
			fmt.Sprintf("automation iteration limit (%d) reached; %d pending items skipped",
				MaxDepth, len(queue)))
	}

	return outcome, nil
}

// processEntityCreations handles entity creation from automation and
// returns new queue items for any newly created entities that have
// their own follow-up automation work.
func (r *Runner) processEntityCreations(
	ctx context.Context,
	host Host,
	trigger *entity.Entity,
	toCreateList []automation.EntityToCreate,
	outcome *Outcome,
) []queueItem {
	var newItems []queueItem

	for _, toCreate := range toCreateList {
		if skip := r.handleIfExists(ctx, host, trigger, toCreate, outcome); skip {
			continue
		}

		// Create entity (no automation yet).
		created, createErr := host.CreateEntityNoCascade(toCreate.Type, CreateEntityOptions{
			TemplateVariant: toCreate.Template,
			Properties:      toCreate.Properties,
		})
		if createErr != nil {
			outcome.Errors = append(outcome.Errors,
				fmt.Sprintf("failed to create automation entity %s: %v", toCreate.Type, createErr))

			continue
		}
		outcome.EntitiesCreated = append(outcome.EntitiesCreated, created)

		// Create relation from trigger if specified.
		if toCreate.RelationFromTrigger != "" {
			r.createTriggerRelation(host, trigger, created, toCreate.RelationFromTrigger, outcome)
		}

		// Run automation on newly created entity.
		newItem := r.runCreatedEntityAutomation(host, created, outcome)
		if newItem != nil {
			newItems = append(newItems, *newItem)
		}
	}

	return newItems
}

// runCreatedEntityAutomation runs automation on a newly created
// entity and returns a queue item if the result implies more work.
func (r *Runner) runCreatedEntityAutomation(
	host Host,
	created *entity.Entity,
	outcome *Outcome,
) *queueItem {
	if r.engine == nil {
		return nil
	}

	newAutoResult := r.engine.Process(automation.Event{
		Type:   automation.EventEntityCreated,
		Entity: created,
	})

	// Apply property changes from automation.
	if len(newAutoResult.PropertiesSet) > 0 {
		for prop, val := range newAutoResult.PropertiesSet {
			created.SetString(prop, val)
		}
		// Re-write entity with updated properties.
		if err := host.WriteEntity(created); err != nil {
			outcome.Errors = append(outcome.Errors,
				fmt.Sprintf("failed to update automation entity %s: %v", created.ID, err))
		}
	}

	// Return queue item if there's more work to do.
	hasWork := len(newAutoResult.EntitiesToCreate) > 0 || len(newAutoResult.RelationsToCreate) > 0 ||
		len(newAutoResult.LuaToExecute) > 0 ||
		len(newAutoResult.Warnings) > 0 || len(newAutoResult.Errors) > 0
	if hasWork {
		return &queueItem{created, newAutoResult}
	}

	return nil
}

// applyRelationCreations creates relations from automation results.
// Each relation's From is rewritten to the trigger entity's ID before
// validation; the To is looked up to ensure the target exists, and
// the (from-type, type, to-type) tuple is validated against the
// metamodel before persisting.
func (r *Runner) applyRelationCreations(
	ctx context.Context,
	host Host,
	triggerEntity *entity.Entity,
	relations []*entity.Relation,
	outcome *Outcome,
) {
	meta := host.Meta()

	for _, rel := range relations {
		rel.From = triggerEntity.ID

		targetEntity, err := host.Store().GetEntity(ctx, rel.To)
		if err != nil {
			outcome.Errors = append(outcome.Errors,
				"automation relation target not found: "+rel.To)
			continue
		}
		if err := meta.ValidateRelation(rel.Type, triggerEntity.Type, targetEntity.Type); err != nil {
			outcome.Errors = append(outcome.Errors,
				fmt.Sprintf("automation relation invalid: %v", err))
			continue
		}

		if err := host.WriteRelation(rel); err != nil {
			outcome.Errors = append(outcome.Errors,
				fmt.Sprintf("failed to create automation relation: %v", err))
			continue
		}
		outcome.RelationsCreated = append(outcome.RelationsCreated, rel)
	}
}

// executeLuaActions executes Lua scripts from automation results.
// Failures are appended to outcome.Errors and the loop continues —
// one bad script does not abort the cascade.
func (r *Runner) executeLuaActions(
	newEntity *entity.Entity,
	oldEntity *entity.Entity,
	luaActions []automation.LuaToExecute,
	deps lua.WriteDeps,
	outcome *Outcome,
) {
	if len(luaActions) == 0 {
		return
	}

	for _, action := range luaActions {
		var err error

		switch {
		case action.Code != "":
			err = r.scripts.ExecuteCode(action.Code, deps, newEntity, oldEntity)
		case action.FilePath != "":
			err = r.scripts.ExecuteFile(action.FilePath, deps, newEntity, oldEntity)
		default:
			// Empty action — skip.
			continue
		}

		if err != nil {
			// Automations have no incoming HTTP request to
			// correlate against, so the slog line uses the
			// automation name + triggering entity id as the
			// natural identity. Operators grep on those rather
			// than a per-request hex.
			triggerID := ""
			if newEntity != nil {
				triggerID = newEntity.ID
			} else if oldEntity != nil {
				triggerID = oldEntity.ID
			}
			slog.Warn("automation script failed",
				"automation", action.AutomationName,
				"entity", triggerID,
				"error", err)
			outcome.Errors = append(outcome.Errors, formatAutomationError(action, err))
		}
	}
}

// formatAutomationError renders an automation Lua failure as a single
// string for the existing []string Errors slice. For Lua failures the
// engine has already wrapped err in *lua.ScriptError; we add the
// automation identity (the engine doesn't know it) and use the typed
// fields so the message includes a concrete script-or-automation name.
//
// Inline `lua: |` blocks have no FilePath, so the engine sees scriptPath
// as "" and tags the envelope with "<inline>". We override that here
// once we know the automation name.
func formatAutomationError(action automation.LuaToExecute, err error) string {
	var se *lua.ScriptError
	if errors.As(err, &se) {
		if action.FilePath == "" && action.AutomationName != "" {
			se.Path = "automation:" + action.AutomationName
			// Re-render Error() output reflects the new Path.
		}
		return se.Error()
	}
	return "script execution error: " + err.Error()
}

// handleIfExists checks if_exists behavior for entity creation.
// Returns true if the entity creation should be skipped (either
// because an existing target was found and the action says skip /
// error, or because an unknown if_exists value was encountered).
func (r *Runner) handleIfExists(
	ctx context.Context,
	host Host,
	triggerEntity *entity.Entity,
	toCreate automation.EntityToCreate,
	outcome *Outcome,
) bool {
	if toCreate.RelationFromTrigger == "" {
		return false
	}

	existingTarget := host.FindExistingRelationTarget(
		triggerEntity.ID, toCreate.RelationFromTrigger, toCreate.Type)

	if existingTarget == nil {
		return false
	}

	switch toCreate.IfExists {
	case automation.IfExistsSkip:
		outcome.EntitiesCreated = append(outcome.EntitiesCreated, existingTarget)
		return true
	case automation.IfExistsError:
		outcome.Errors = append(outcome.Errors,
			fmt.Sprintf("entity already exists via %s relation: %s",
				toCreate.RelationFromTrigger, existingTarget.ID))
		return true
	case automation.IfExistsReplace:
		if err := host.DeleteEntity(ctx, existingTarget.Type, existingTarget.ID, true); err != nil {
			outcome.Errors = append(outcome.Errors,
				fmt.Sprintf("failed to delete existing entity for replace: %v", err))
			return true
		}
	default:
		outcome.Errors = append(outcome.Errors,
			fmt.Sprintf("unknown if_exists value %q, skipping entity creation", toCreate.IfExists))
		return true
	}
	return false
}

// createTriggerRelation creates a relation from the trigger entity to
// a newly created entity. Failures are appended to outcome.Errors.
func (r *Runner) createTriggerRelation(
	host Host,
	triggerEntity, created *entity.Entity,
	relationType string,
	outcome *Outcome,
) {
	meta := host.Meta()

	if err := meta.ValidateRelation(relationType, triggerEntity.Type, created.Type); err != nil {
		outcome.Errors = append(outcome.Errors,
			fmt.Sprintf("automation relation invalid: %v", err))
		return
	}

	rel := entity.NewRelation(triggerEntity.ID, relationType, created.ID)
	if err := host.WriteRelation(rel); err != nil {
		outcome.Errors = append(outcome.Errors,
			fmt.Sprintf("failed to create automation relation: %v", err))
		return
	}
	outcome.RelationsCreated = append(outcome.RelationsCreated, rel)
}
