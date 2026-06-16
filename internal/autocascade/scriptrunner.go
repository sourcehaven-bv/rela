package autocascade

import (
	"context"

	"github.com/Sourcehaven-BV/rela/internal/entity"
)

// ScriptRunner is the abstraction Runner uses to execute scripted
// automation actions. It is intentionally script-runtime-agnostic:
// Runner does not know whether the underlying engine is Lua,
// JavaScript, or something else.
//
// Lifecycle: ScriptRunner is built once at wiring time. Each Run call
// receives the per-cascade [Mutator] from [Request.Mutator] so script
// actions can call back into the graph (create / update / delete).
// This per-call mutator is the contract that lets ScriptRunner remain
// free of the construction-time cycle between EntityManager and the
// engine's write-deps assembly.
type ScriptRunner interface {
	// Run executes the action and returns any error from the
	// underlying engine. Implementations are responsible for any
	// engine-specific error formatting (e.g. patching automation
	// names into Lua script-error envelopes) — Runner appends the
	// stringified error to Outcome.Errors as-is and continues the
	// cascade.
	//
	// mutator is the per-cascade write handle the script may invoke;
	// engines that don't expose mutation to scripts may ignore it.
	Run(ctx context.Context, action ScriptAction, mutator Mutator) error
}

// ScriptAction is one scripted automation action passed to a
// [ScriptRunner]. Code and FilePath are mutually exclusive; at most
// one is non-empty. NewEntity is the cascade-current trigger entity;
// OldEntity is the original trigger's prior state (or nil for
// creates). Name is the automation identity for error attribution.
type ScriptAction struct {
	// Code is inline script source. Mutually exclusive with FilePath.
	Code string

	// FilePath is the path to a script file, resolved by the
	// underlying engine. Mutually exclusive with Code.
	FilePath string

	// Name is the automation that emitted this action, used by
	// implementations for error attribution.
	Name string

	// NewEntity is the entity context Runner is currently
	// processing. May be the original trigger (top of cascade) or a
	// cascaded creation (deeper iterations).
	NewEntity *entity.Entity

	// OldEntity is the original trigger's prior state (nil for
	// creates). Note that during cascades this carries the *original*
	// trigger's old state, not the current iteration's — preserved
	// from pre-refactor workspace behavior.
	OldEntity *entity.Entity

	// AllowACLBypass mirrors the action's `allow_acl_bypass` flag
	// (TKT-D8T148). When true, the script runner exposes `rela.bypass_acl`
	// backed by an elevated Mutator (from [ElevatedProvider]); when false the
	// binding is absent and the script cannot elevate. Operator-gated: only a
	// metamodel-authored action can set it.
	AllowACLBypass bool
}

// NopScriptRunner is a no-op [ScriptRunner] for tests that should not
// trigger script execution. It panics when called, making it obvious
// when a test unexpectedly fires a scripted automation.
var NopScriptRunner ScriptRunner = nopScriptRunner{}

type nopScriptRunner struct{}

func (nopScriptRunner) Run(_ context.Context, _ ScriptAction, _ Mutator) error {
	panic("autocascade.NopScriptRunner: script execution not expected in this context")
}
