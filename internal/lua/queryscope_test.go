package lua

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// scopedTicketMeta is testMeta plus a DEFAULT query scope hiding archived
// rows. `~=` does not lower to a store predicate, so nothing but a deliberate
// Go-side filter could drop the row.
func scopedTicketMeta() *metamodel.Metamodel {
	m := testMeta()
	def := m.Entities["ticket"]
	def.QueryScopes = map[string]string{
		"default": "entity.status ~= 'gearchiveerd'",
		"archief": "entity.status == 'gearchiveerd'",
	}
	m.Entities["ticket"] = def
	return m
}

// TestListEntities_IgnoresQueryScopes is AC6 for Lua (TKT-EVR2TU).
//
// A script reads the graph to compute something, not to render a screen.
// Inheriting the schema's default scope would narrow every script on a type
// from a declaration the script never mentions — an automation totalling open
// work would silently stop counting archived rows, and the script would look
// correct. The user's framing: applying it by default is "too surprising for
// script authors".
//
// Opting IN by naming a scope is AC7, decided separately.
func TestListEntities_IgnoresQueryScopes(t *testing.T) {
	ws := newMockWorkspaceWith(scopedTicketMeta())
	for _, tc := range []struct{ id, status string }{
		{"TKT-1", "open"},
		{"TKT-2", "gearchiveerd"},
	} {
		e := entity.New(tc.id, "ticket")
		e.SetString("title", tc.id)
		e.SetString("status", tc.status)
		ws.seedEntity(e)
	}

	var buf bytes.Buffer
	r := NewWriter(ws.services(t.TempDir()), &buf)
	defer r.Close()

	const src = `
local rows = rela.list_entities("ticket")
local ids = {}
for _, e in ipairs(rows) do ids[#ids+1] = e.id end
table.sort(ids)
rela.output({ids = ids})
`
	if err := r.RunString(src); err != nil {
		t.Fatalf("RunString: %v", err)
	}
	var out struct {
		IDs []string `json:"ids"`
	}
	if err := json.Unmarshal(buf.Bytes(), &out); err != nil {
		t.Fatalf("decode %q: %v", buf.String(), err)
	}

	want := []string{"TKT-1", "TKT-2"}
	if len(out.IDs) != len(want) {
		t.Fatalf("Lua list_entities returned %v, want %v — a schema default query "+
			"scope must not narrow a script read", out.IDs, want)
	}
	for i := range want {
		if out.IDs[i] != want[i] {
			t.Fatalf("got %v, want %v", out.IDs, want)
		}
	}
}
