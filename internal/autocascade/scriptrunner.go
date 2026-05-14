package autocascade

import (
	"context"

	"github.com/Sourcehaven-BV/rela/internal/entity"
)

// ScriptRunner is the per-request abstraction Runner uses to execute
// scripted automation actions. It is intentionally script-runtime-
// agnostic: Runner does not know whether the underlying engine is
// Lua, JavaScript, or something else.
//
// The caller of [Runner.Process] constructs a per-call ScriptRunner
// that has already bound any per-request state the engine needs
// (capability bundles, entity context, audit metadata, etc.) — Runner
// just hands it a [ScriptAction] and asks for execution.
//
// Lifecycle: ScriptRunner is request-scoped. The caller may freshly
// construct one per Process call (typical for Lua, where deps need to
// be assembled per-request) or reuse one across requests (cheap when
// no per-request state needs binding).
type ScriptRunner interface {
	// Run executes the action and returns any error from the
	// underlying engine. Implementations are responsible for any
	// engine-specific error formatting (e.g. patching automation
	// names into Lua script-error envelopes) — Runner appends the
	// stringified error to Outcome.Errors as-is and continues the
	// cascade.
	Run(ctx context.Context, action ScriptAction) error
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
}

// NopScriptRunner is a no-op [ScriptRunner] for tests that should not
// trigger script execution. It panics when called, making it obvious
// when a test unexpectedly fires a scripted automation.
var NopScriptRunner ScriptRunner = nopScriptRunner{}

type nopScriptRunner struct{}

func (nopScriptRunner) Run(_ context.Context, _ ScriptAction) error {
	panic("autocascade.NopScriptRunner: script execution not expected in this context")
}
