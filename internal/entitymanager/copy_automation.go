package entitymanager

import (
	"context"
	"fmt"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/autocascade"
	"github.com/Sourcehaven-BV/rela/internal/automation"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// A copy runs the target face's automations (TKT-50DZEHI). A copy that
// creates the face raises a created event, and one that overwrites it raises
// an updated event against the face as it was before. The event names the
// copy, so `on.copy:` can select it.
//
// It follows the update path: [copyEngine.applyCopyAutomations] computes the
// automations before the copy's transaction and folds the properties they
// set into the target the copy writes, so the copy stays one write with one
// audit record and its provenance. [copyEngine.runCopyCascade] runs the
// side effects after the commit.

// applyCopyAutomations evaluates the automations for the planned target face
// and writes the properties they set onto plan.entity. Those values then pass
// the checks an update applies to automation output: no computed property or
// external ref, hard validation, and the state machine. The state machine
// checks the move from the copied values to the automation's values only;
// the copy's own values are a copy boundary, not a transition.
//
// The returned result is empty when no automation engine is wired.
func (ce *copyEngine) applyCopyAutomations(ctx context.Context, plan *copyPlan) (*automation.Result, error) {
	m := ce.m
	if m.deps.Automations == nil {
		return &automation.Result{}, nil
	}
	ev := automation.Event{Type: automation.EventEntityCreated, Entity: plan.entity, Copy: plan.name}
	if !plan.created {
		ev.Type, ev.OldEntity = automation.EventEntityUpdated, plan.existing
	}
	res := m.deps.Automations.Process(ctx, ev)
	if len(res.PropertiesSet) == 0 {
		return res, nil
	}
	set := stringMapAny(res.PropertiesSet)
	if err := rejectComputedPresent(m.deps, plan.entity.Type, set); err != nil {
		return nil, err
	}
	if err := rejectExternalRefPresent(m.deps.Meta, plan.entity.Type, set); err != nil {
		return nil, err
	}
	copied := plan.entity.Clone()
	for prop, val := range res.PropertiesSet {
		plan.entity.SetString(prop, val)
	}
	if err := m.deps.Computed.Evaluate(ctx, plan.entity); err != nil {
		return nil, err
	}
	// The copy writes plan.entity as it stands now, so the validation mode
	// and the rules enforced on write apply to this state, as
	// buildCopyTarget applied them to the state before the automations.
	hard, _ := partitionValidationErrors(m.deps.ValidationMode,
		metamodel.NewValidator(m.deps.Meta).ValidateEntity(plan.entity.ID, plan.entity.Type, plan.entity.Properties))
	if len(hard) > 0 {
		return nil, newValidationError(hard)
	}
	if err := checkWriteRules(ctx, m.deps, plan.entity); err != nil {
		return nil, err
	}
	if err := m.deps.Transitions.EnforceUpdate(
		ctx, copied, plan.entity, m.deps.TransitionGuard, m.deps.TransitionGraph,
	); err != nil {
		return nil, m.mapTransitionError(
			ctx, acl.NewEntitySubject(plan.entity.Type, plan.entity.ID, plan.entity.Face), err)
	}
	return res, nil
}

// runCopyCascade runs the side effects of the automations after the copy has
// committed: relations, entities, scripts, background jobs, piles and
// notifications. It runs through the gated mutator, so elevation never
// reaches a nested write. On an error the copy stays written, and the caller
// reports it as [CopyResult.FollowUpErr].
func (ce *copyEngine) runCopyCascade(
	ctx context.Context, plan *copyPlan, autoResult *automation.Result, res *CopyResult,
) error {
	m := ce.m
	if m.deps.Cascade == nil || autoResult == nil || res.Entity == nil {
		return nil
	}
	res.AutomationWarnings = autoResult.Warnings
	res.AutomationErrors = autoResult.Errors
	outcome, err := m.deps.Cascade.Process(ctx, &cascadeHost{deps: m.deps}, autocascade.Request{
		Trigger:    res.Entity,
		OldTrigger: plan.existing,
		Result:     autoResult,
		Scripts:    m.deps.ScriptRunner,
		Mutator:    m.gated(), // gated() so elevation never propagates into a nested cascade (TKT-D8T148)
	})
	if err != nil {
		return fmt.Errorf("entitymanager: copy %q: cascade: %w", plan.name, err)
	}
	res.AutomationErrors = append(res.AutomationErrors, outcome.Errors...)
	res.AutomationWarnings = append(res.AutomationWarnings, outcome.Warnings...)
	return nil
}
