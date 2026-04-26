package dataentry

import (
	"encoding/json"
	"net"
	"net/http"

	"github.com/Sourcehaven-BV/rela/internal/lua"
)

// ScriptErrorEnvelope is the on-the-wire representation of a Lua script
// failure. The HTTP layer always returns this with status 422 so the
// frontend can branch on `error == "script_error"`. Every field except
// `correlation_id`, `script.path`, and `lua.message` is loopback-gated:
// non-loopback callers receive a degraded shape unless the operator has
// explicitly opted in to full detail (see ScriptErrorPolicy).
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

// ScriptErrorPolicy decides how much detail crosses the wire. The
// constructor honors both the request peer (loopback callers always
// get full detail) and an operator opt-in for non-loopback peers.
//
// Off-by-default for non-loopback because the data-entry server has no
// auth layer; rich envelopes would otherwise leak script source and
// captured print() output across the LAN.
type ScriptErrorPolicy struct {
	// AlwaysFullDetail bypasses the loopback check. Set to true via
	// data-entry.yaml when the operator knows the deployment is safe.
	AlwaysFullDetail bool
}

// allowFullDetail reports whether the caller of r should receive the
// rich envelope (source slice, full stack, captured output).
//
// The decision is intentionally based on r.RemoteAddr only — no
// X-Forwarded-For honoring. Behind a reverse proxy this fails closed
// (proxy IP is non-loopback → degraded shape), which is the right
// default since the data-entry server has no auth layer. Anyone adding
// proxy-aware middleware later must keep this gate honest.
func (p ScriptErrorPolicy) allowFullDetail(r *http.Request) bool {
	if p.AlwaysFullDetail {
		return true
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	return isLoopback(host)
}

// writeV1ScriptError renders a *lua.ScriptError as the structured
// envelope. Always 422: the request was understood, the user-supplied
// script was the problem.
func writeV1ScriptError(w http.ResponseWriter, r *http.Request, se *lua.ScriptError, policy ScriptErrorPolicy) {
	env := ScriptErrorEnvelope{
		Error:         "script_error",
		CorrelationID: se.CorrelationID,
		Script: ScriptIdentity{
			Surface:  se.Surface,
			Path:     se.Path,
			EntityID: se.EntityID,
			Args:     se.Args,
		},
		Lua: ScriptErrorLua{
			Message: se.LuaMessage,
			Line:    se.LuaLine,
		},
	}
	if policy.allowFullDetail(r) {
		env.Source = se.Source
		env.Stack = se.Stack
		env.CapturedOutput = se.CapturedOutput
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	w.WriteHeader(http.StatusUnprocessableEntity)
	_ = json.NewEncoder(w).Encode(env)
}
