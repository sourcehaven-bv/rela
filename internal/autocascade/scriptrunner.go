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

	// AllowACLBypass mirrors the action's `allow_acl_bypass` (TKT-D8T148,
	// TKT-Y3JVFK). When set, the script runner exposes `rela.bypass_acl`
	// backed by the capabilities the value names — an elevated Mutator (from
	// [ElevatedProvider]) for write, an elevated reader for read; when unset
	// the binding is absent and the script cannot elevate. Operator-gated:
	// only a schema-authored action can set it.
	//
	// Typed as a plain string rather than metamodel.ACLBypass because this
	// package is deliberately schema-agnostic (it may not import metamodel —
	// see .go-arch-lint.yml). The caller converts; the values are the
	// metamodel.ACLBypass* constants and AllowsRead/AllowsWrite below mirror
	// their semantics.
	AllowACLBypass string

	// Capabilities mirrors the action's `capabilities:` block (TKT-YH52OM):
	// which ambient, non-graph capabilities the script may reach. The zero
	// value grants NOTHING — an automation runs on the write path of any HTTP
	// request, so it is not an operator-shell surface.
	//
	// Carried as primitives for the same reason AllowACLBypass is a string:
	// this package may not import metamodel or lua. The caller converts.
	Capabilities ScriptCapabilities
}

// ScriptCapabilities is the schema-agnostic carrier for a scripted action's
// ambient capability grants. It mirrors metamodel.Capabilities / lua.Capabilities
// field-for-field; see [ScriptAction.Capabilities] for why it is duplicated
// rather than imported.
type ScriptCapabilities struct {
	HTTP bool
	AI   bool
	// Mail authorizes mail.send. Unlike the others, absence does not remove
	// the binding — it makes it refuse; see lua.Capabilities.Mail.
	Mail      bool
	WriteFile bool
	// Secrets names the .rela/secrets.yaml keys the script may read. Empty
	// means none — NOT all.
	Secrets []string
}

// Fields returns the grant as plain values, mirroring
// metamodel.Capabilities.Fields so both ends of the hop read through one shape.
func (c ScriptCapabilities) Fields() (http, ai, mail, writeFile bool, secrets []string) {
	return c.HTTP, c.AI, c.Mail, c.WriteFile, c.Secrets
}

// NopScriptRunner is a no-op [ScriptRunner] for tests that should not
// trigger script execution. It panics when called, making it obvious
// when a test unexpectedly fires a scripted automation.
var NopScriptRunner ScriptRunner = nopScriptRunner{}

type nopScriptRunner struct{}

func (nopScriptRunner) Run(_ context.Context, _ ScriptAction, _ Mutator) error {
	panic("autocascade.NopScriptRunner: script execution not expected in this context")
}

// Elevation capability values carried by [ScriptAction.AllowACLBypass].
// These MUST match the metamodel.ACLBypass* constants; the duplication is
// what keeps this package free of a schema dependency.
const (
	ACLBypassRead      = "read"
	ACLBypassWrite     = "write"
	ACLBypassReadWrite = "read+write"
)
