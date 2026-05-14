package autocascade

import (
	"github.com/Sourcehaven-BV/rela/internal/automation"
	"github.com/Sourcehaven-BV/rela/internal/entity"
)

// MaxDepth is the cascade depth limit. When [Runner.Process] reaches
// this many BFS iterations with queue items still pending, it stops
// and emits a warning naming the depth and the count of unprocessed
// items. The constant is exported so callers can verify against it
// in tests and observability.
//
// 50 was chosen empirically: deep enough that no realistic
// project-level automation chain hits it, shallow enough that
// pathological loops fail loudly rather than hanging.
const MaxDepth = 50

// Request is the per-invocation payload Runner.Process needs.
// Grouped into a struct to keep the Process signature readable as
// more fields are added (audit context, principal, etc.).
type Request struct {
	// Trigger is the entity whose write initiated the cascade.
	Trigger *entity.Entity

	// OldTrigger is the trigger's prior state (nil for creates).
	//
	// Note: OldTrigger is passed through to scripts for every queue
	// item, not just the initial one. Cascaded entities (created
	// later in the cascade) therefore see the *original* trigger's
	// old state, not their own (which would be nil anyway). This is
	// pre-existing behavior preserved from
	// workspace.applyAutomationSideEffects.
	OldTrigger *entity.Entity

	// Result is the automation.Result produced by
	// [automation.Engine.Process] for the initiating event.
	Result *automation.Result

	// Scripts is the per-call script execution adapter. The caller
	// constructs one with any per-request state (capability
	// bundles, audit context) already bound, so Runner does not
	// need to know about the underlying engine. Optional: when nil,
	// scripted automation actions are recorded as errors in the
	// Outcome (rather than silently skipped) — production callers
	// always supply one.
	Scripts ScriptRunner
}

// Outcome is the cumulative result of a cascade.
//
// Errors is []string for wire-format symmetry with the existing
// EntityManager result types (UpdateResult.AutomationErrors etc.),
// which the API layer reads as text. A typed []error variant
// preserving *lua.ScriptError will arrive when a consumer needs
// structured access; until then, accept the stringification.
type Outcome struct {
	RelationsCreated []*entity.Relation
	EntitiesCreated  []*entity.Entity
	Errors           []string
	Warnings         []string
}
