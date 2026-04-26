package dataentry

import (
	"encoding/json"
	"net/http"

	"github.com/Sourcehaven-BV/rela/internal/lua"
)

// ScriptErrorEnvelope is the on-the-wire representation of a Lua script
// failure. The HTTP layer always returns this with status 422 so the
// frontend can branch on `error == "script_error"`. Every field except
// `correlation_id`, `script.path`, and `lua.message` is loopback-gated:
// non-loopback callers receive a degraded shape (see
// security.AllowFullScriptDetail).
//
// JSON tags on lua.ScriptError are advisory: each consumer owns its
// wire shape. Data-entry uses this envelope; MCP marshals lua.ScriptError
// directly. Adding a field to lua.ScriptError requires deciding whether
// to mirror it here or let it surface only over MCP.
type ScriptErrorEnvelope struct {
	Error          string           `json:"error"`
	CorrelationID  string           `json:"correlation_id,omitempty"`
	Script         ScriptIdentity   `json:"script"`
	Lua            ScriptErrorLua   `json:"lua"`
	Source         []lua.SourceLine `json:"source,omitempty"`
	Stack          []lua.StackFrame `json:"stack,omitempty"`
	CapturedOutput string           `json:"captured_output,omitempty"`
}

// ScriptIdentity carries who-was-running info: the surface (action,
// document, automation, lua_run, lua_eval), the script path, and any
// triggering entity / args context the surface decided to capture.
type ScriptIdentity struct {
	Surface  string         `json:"surface"`
	Path     string         `json:"path"`
	EntityID string         `json:"entity_id,omitempty"`
	Args     map[string]any `json:"args,omitempty"`
}

// ScriptErrorLua is the message + line that always survives gating, so
// even a non-loopback caller knows roughly what broke without leaking
// the full source slice.
type ScriptErrorLua struct {
	Message string `json:"message"`
	Line    int    `json:"line,omitempty"`
}

// allowFullScriptDetail asks the App's security layer whether r is
// trusted enough for the rich envelope. Defaults to false when no
// security has been wired (unit tests that bypass NewRouter), which
// keeps tests honest about exercising the gate.
func (a *App) allowFullScriptDetail(r *http.Request) bool {
	if a.security == nil {
		return false
	}
	return a.security.AllowFullScriptDetail(r)
}

// buildScriptErrorEnvelope renders a *lua.ScriptError as the structured
// wire envelope. Shared by the action surface (writeV1ScriptError) and
// the analyze surface (which embeds the envelope inside an AnalysisIssue
// instead of returning it as the top-level body).
//
// fullDetail decides whether the gated fields (Source, Stack,
// CapturedOutput) cross the wire. The caller obtains it from
// security.AllowFullScriptDetail so the loopback decision lives next
// to the rest of the host-trust policy.
//
// correlationID overrides whatever the engine stamped on the
// *ScriptError — important because singleflight may hand the same
// *ScriptError to multiple requests, and each one needs its own id in
// the response. Pass "" to fall back to se.CorrelationID.
func buildScriptErrorEnvelope(se *lua.ScriptError, fullDetail bool, correlationID string) ScriptErrorEnvelope {
	corrID := correlationID
	if corrID == "" {
		corrID = se.CorrelationID
	}
	env := ScriptErrorEnvelope{
		Error:         "script_error",
		CorrelationID: corrID,
		Script: ScriptIdentity{
			Surface:  string(se.Surface),
			Path:     se.Path,
			EntityID: se.EntityID,
			Args:     se.Args,
		},
		Lua: ScriptErrorLua{
			Message: se.LuaMessage,
			Line:    se.LuaLine,
		},
	}
	if fullDetail {
		env.Source = se.Source
		env.Stack = se.Stack
		env.CapturedOutput = se.CapturedOutput
	}
	return env
}

// writeV1ScriptError renders a *lua.ScriptError as the structured
// envelope. Always 422: the request was understood, the user-supplied
// script was the problem.
//
// See buildScriptErrorEnvelope for the gating + correlation-id rules.
func writeV1ScriptError(w http.ResponseWriter, se *lua.ScriptError, fullDetail bool, correlationID string) {
	env := buildScriptErrorEnvelope(se, fullDetail, correlationID)
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	w.WriteHeader(http.StatusUnprocessableEntity)
	_ = json.NewEncoder(w).Encode(env)
}
