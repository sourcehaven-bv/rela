package lua

import (
	"bytes"
	"strings"
	"testing"
)

// TestAuditNotExposedInLuaAPI is the spoofing defense from PLAN-XKMJ
// AC13. A Lua writer runtime exposes no `audit` table on `rela.*`;
// scripts cannot rewrite their own Principal or triggered_by
// attribution. If a future change accidentally registers an audit
// binding, this test fails — a deliberate stop the gate.
func TestAuditNotExposedInLuaAPI(t *testing.T) {
	t.Parallel()
	ws := newMockWorkspace(t)
	var buf bytes.Buffer
	r := NewWriter(ws.services("/tmp"), &buf)
	defer r.Close()

	// type() is nil-safe and stays valid through chained-nil access if
	// we type-check the parent first.
	probes := []struct {
		name string
		expr string
	}{
		// rela.audit must not exist
		{"rela.audit", `type(rela.audit) ~= "nil"`},
		// rela.principal must not exist
		{"rela.principal", `type(rela.principal) ~= "nil"`},
		// guarded chained probes — only fire if the parent is a table
		{"rela.audit.with_principal", `type(rela.audit) == "table" and rela.audit.with_principal ~= nil`},
		{"rela.audit.with_triggered_by", `type(rela.audit) == "table" and rela.audit.with_triggered_by ~= nil`},
	}

	for _, probe := range probes {
		t.Run(probe.name, func(t *testing.T) {
			script := `if (` + probe.expr + `) then error("found") end`
			if err := r.RunString(script); err != nil {
				t.Errorf("audit binding %q is reachable from Lua: %v", probe.name, err)
			}
		})
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
