package autocascade

import (
	"github.com/Sourcehaven-BV/rela/internal/automation"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/lua"
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
	// Note: OldTrigger is passed through to Lua scripts for every
	// queue item, not just the initial one. Cascaded entities
	// (created later in the cascade) therefore see the *original*
	// trigger's old state, not their own (which would be nil
	// anyway). This is pre-existing behavior preserved from
	// workspace.applyAutomationSideEffects; see comment in
	// runner.go's executeLuaActions for context.
	OldTrigger *entity.Entity

	// Result is the automation.Result produced by
	// [automation.Engine.Process] for the initiating event.
	Result *automation.Result

	// LuaDeps is the WriteDeps bundle passed through to script
	// execution. The caller materializes it; Runner just hands it
	// to the script executor. Lives on Request rather than on Host
	// because future cycle resolution (see TKT-Y0JU) may require
	// callers to assemble it specially per-invocation; deferring
	// that materialization to Host would entangle Host with a
	// lua-imported type.
	LuaDeps lua.WriteDeps
}

// Outcome is the cumulative result of a cascade. Field order mirrors
// workspace.automationSideEffects: RelationsCreated, EntitiesCreated,
// Errors, Warnings.
//
// Errors is intentionally []string rather than []error. Today no
// consumer branches on the underlying type; UpdateResult.AutomationErrors
// is read only as text by the API layer. Promote to []error
// (preserving *lua.ScriptError) when a future surface needs typed
// access — at which point formatAutomationError's stringification
// goes away.
type Outcome struct {
	RelationsCreated []*entity.Relation
	EntitiesCreated  []*entity.Entity
	Errors           []string
	Warnings         []string
}
