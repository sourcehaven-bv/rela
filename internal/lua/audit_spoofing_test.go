package lua

import (
	"bytes"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/principal"
)

// TestAuditNotExposedInLuaAPI is the spoofing defense from PLAN-XKMJ
// AC13. A Lua writer runtime exposes no audit *rewrite* surface: scripts
// cannot change the Principal or triggered_by a write is attributed as.
// The attribution always derives from the caller's context inside the
// write bindings, never from anything the script controls. If a future
// change registers an audit-rewrite binding, this test fails.
//
// Note (TKT-5U6NRR): `rela.principal` IS exposed — but read-only, and only
// as a *read* of the current identity, not a rewrite hook. Its read-only
// contract and the can't-forge invariant are pinned by
// TestPrincipalIsReadOnly / TestPrincipalCannotForgeAttribution below; the
// rewrite vectors (`rela.audit*`) remain absent here.
func TestAuditNotExposedInLuaAPI(t *testing.T) {
	t.Parallel()

	// type() is nil-safe and stays valid through chained-nil access if
	// we type-check the parent first.
	probes := []struct {
		name string
		expr string
	}{
		// rela.audit must not exist — it would be a rewrite table.
		{"rela.audit", `type(rela.audit) ~= "nil"`},
		// guarded chained probes — only fire if the parent is a table
		{"rela.audit.with_principal", `type(rela.audit) == "table" and rela.audit.with_principal ~= nil`},
		{"rela.audit.with_triggered_by", `type(rela.audit) == "table" and rela.audit.with_triggered_by ~= nil`},
	}

	for _, probe := range probes {
		t.Run(probe.name, func(t *testing.T) {
			t.Parallel()
			// Each parallel subtest needs its own runtime: a shared
			// LState is not goroutine-safe, and a parent-level
			// `defer r.Close()` would run before parallel subtests do.
			ws := newMockWorkspace(t)
			var buf bytes.Buffer
			r := NewWriter(ws.services("/tmp"), &buf)
			defer r.Close()

			script := `if (` + probe.expr + `) then error("found") end`
			if err := r.RunString(script); err != nil {
				t.Errorf("audit binding %q is reachable from Lua: %v", probe.name, err)
			}
		})
	}
}

// TestPrincipalIsReadOnly pins the TKT-5U6NRR contract: rela.principal is
// readable (so write-path automations can attribute relations to the acting
// user) but FROZEN — any assignment to it raises, so the read-only guarantee
// is enforced rather than conventional. setmetatable cannot swap the guard out
// either (__metatable is locked).
func TestPrincipalIsReadOnly(t *testing.T) {
	t.Parallel()
	ws := newMockWorkspace(t)
	var buf bytes.Buffer
	r := NewWriter(ws.services("/tmp"), &buf)
	defer r.Close()

	// Readable: the field exists and has the expected shape.
	if err := r.RunString(`
		assert(type(rela.principal) == "table", "rela.principal should be a table")
		assert(type(rela.principal.user) == "string", "rela.principal.user should be a string")
		assert(type(rela.principal.tool) == "string", "rela.principal.tool should be a string")
	`); err != nil {
		t.Errorf("rela.principal not readable as expected: %v", err)
	}

	// Frozen: assigning a new key raises.
	if err := r.RunString(`rela.principal.user = "mallory"`); err == nil {
		t.Error("expected assignment to rela.principal to raise (read-only), but it succeeded")
	}

	// The metatable is locked, so it can't be swapped to bypass the guard.
	if err := r.RunString(`setmetatable(rela.principal, {})`); err == nil {
		t.Error("expected setmetatable on rela.principal to raise (locked __metatable), but it succeeded")
	}
}

// TestAuditNotExposedAsTopLevelGlobal ensures audit isn't accidentally
// registered at the global table level either (some bindings live at
// `audit.foo()` rather than `rela.audit.foo()`).
func TestAuditNotExposedAsTopLevelGlobal(t *testing.T) {
	t.Parallel()
	ws := newMockWorkspace(t)
	var buf bytes.Buffer
	r := NewWriter(ws.services("/tmp"), &buf)
	defer r.Close()

	script := `if type(audit) ~= "nil" then error("audit global exists: " .. type(audit)) end`
	if err := r.RunString(script); err != nil {
		if strings.Contains(err.Error(), "audit global exists") {
			t.Errorf("audit is a reachable global from Lua — spoofing vector")
		} else {
			t.Fatalf("unexpected error: %v", err)
		}
	}
}

// TestPrincipalReflectsContext pins the core TKT-5U6NRR acceptance: a write-path
// runtime's rela.principal.user / .tool equal the principal the caller passed
// via WithPrincipal (the X-Rela-User-derived identity), not the git user. This
// is what lets a submit-time automation attribute a created-by edge to the
// actual submitter.
func TestPrincipalReflectsContext(t *testing.T) {
	t.Parallel()
	ws := newMockWorkspace(t)
	var buf bytes.Buffer
	r := NewWriter(ws.services("/tmp"), &buf,
		WithPrincipal(principal.Principal{User: "alice", Tool: principal.ToolDataEntry}),
		WithActionMode())
	defer r.Close()

	ret, err := r.RunActionString(`return rela.principal.user`, "test.lua")
	if err != nil {
		t.Fatalf("RunActionString: %v", err)
	}
	if ret != "alice" {
		t.Errorf("rela.principal.user = %v, want \"alice\" (the ctx principal)", ret)
	}

	ret, err = r.RunActionString(`return rela.principal.tool`, "test.lua")
	if err != nil {
		t.Fatalf("RunActionString: %v", err)
	}
	if ret != principal.ToolDataEntry {
		t.Errorf("rela.principal.tool = %v, want %q", ret, principal.ToolDataEntry)
	}
}

// TestPrincipalFallsBackToUnknown pins the documented fallback: an unstamped
// context (CLI / scheduler with no principal) yields the "unknown" value from
// principal.From rather than an error or empty string, so scripts can always
// read the field safely.
func TestPrincipalFallsBackToUnknown(t *testing.T) {
	t.Parallel()
	ws := newMockWorkspace(t)
	var buf bytes.Buffer
	// No WithPrincipal → the zero Principal → the unknown fallback.
	r := NewWriter(ws.services("/tmp"), &buf, WithActionMode())
	defer r.Close()

	ret, err := r.RunActionString(`return rela.principal.user`, "test.lua")
	if err != nil {
		t.Fatalf("RunActionString: %v", err)
	}
	if ret != "unknown" {
		t.Errorf("rela.principal.user on unstamped ctx = %v, want \"unknown\"", ret)
	}
}
