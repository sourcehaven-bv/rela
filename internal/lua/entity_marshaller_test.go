package lua

import (
	"sort"
	"testing"

	lua "github.com/yuin/gopher-lua"

	"github.com/Sourcehaven-BV/rela/internal/entity"
)

// runScriptWithRecordingEntity marshals e into the `entity` global via a
// recording marshaller, runs script, and returns the recorded field set plus
// any Lua error. It exercises the exact seam the validator will use.
func runScriptWithRecordingEntity(t *testing.T, e *entity.Entity, script string) ([]string, error) {
	t.Helper()
	ls := lua.NewState()
	defer ls.Close()

	m, rec := NewRecordingMarshaller()
	ls.SetGlobal("entity", m(ls, e))
	if err := ls.DoString(script); err != nil {
		return nil, err
	}
	got := rec.AccessedFor(e.ID)
	sort.Strings(got)
	return got, nil
}

func sampleEntity() *entity.Entity {
	return &entity.Entity{
		ID:   "TKT-001",
		Type: "ticket",
		Properties: map[string]any{
			"title":    "First",
			"status":   "open",
			"priority": "high",
		},
		Content: "body text",
	}
}

func TestRecordingMarshaller_CapturesEachAccessPath(t *testing.T) {
	tests := []struct {
		name   string
		script string
		want   []string
	}{
		{
			name:   "direct property read via .properties.X",
			script: `local _ = entity.properties.status`,
			want:   []string{"status"},
		},
		{
			name:   "multiple direct reads",
			script: `local _ = entity.properties.status; local _ = entity.properties.priority`,
			want:   []string{"priority", "status"},
		},
		{
			name:   "index syntax .properties[k]",
			script: `local k = "title"; local _ = entity.properties[k]`,
			want:   []string{"title"},
		},
		{
			name:   "prop(name) method records name",
			script: `local _ = entity:prop("status")`,
			want:   []string{"status"},
		},
		{
			name:   "prop with default on absent key still records the touched name",
			script: `local _ = entity:prop("nope", "fallback")`,
			want:   []string{"nope"},
		},
		{
			name:   "content read records the content sentinel",
			script: `local _ = entity.content`,
			want:   []string{FieldContent},
		},
		{
			name:   "id/type/mod_time reads are NOT recorded (never ACL-hideable)",
			script: `local _ = entity.id; local _ = entity.type; local _ = entity.mod_time`,
			want:   []string{},
		},
		{
			name:   "reading nothing records nothing",
			script: `local x = 1 + 1`,
			want:   []string{},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := runScriptWithRecordingEntity(t, sampleEntity(), tc.script)
			if err != nil {
				t.Fatalf("script error: %v", err)
			}
			if !equalStringSets(got, tc.want) {
				t.Errorf("recorded = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestRecordingMarshaller_ServesRealValues proves the gate does not break the
// script contract: a recorded read returns the ACTUAL property value, so a
// validation rule behaves identically to the eager marshaller for the reads it
// makes.
func TestRecordingMarshaller_ServesRealValues(t *testing.T) {
	ls := lua.NewState()
	defer ls.Close()
	m, _ := NewRecordingMarshaller()
	ls.SetGlobal("entity", m(ls, sampleEntity()))

	if err := ls.DoString(`
		assert(entity.properties.status == "open", "direct read value")
		assert(entity:prop("priority") == "high", "prop() value")
		assert(entity.content == "body text", "content value")
		assert(entity.id == "TKT-001", "id value")
		assert(entity:prop("missing", "dflt") == "dflt", "prop default")
	`); err != nil {
		t.Fatalf("value assertions failed: %v", err)
	}
}

// TestRecordingMarshaller_PairsIterationBehaviour MEASURES the documented
// gopher-lua caveat: pairs() iterates raw table contents and does not fire
// __index, so iterating the empty proxy records NOTHING and sees NO properties.
// This test pins the observed behavior so the consumer policy (treat an
// iterating rule conservatively) is grounded in a measured fact, not a guess.
func TestRecordingMarshaller_PairsIterationBehaviour(t *testing.T) {
	got, err := runScriptWithRecordingEntity(t, sampleEntity(), `
		local count = 0
		for k, v in pairs(entity.properties) do count = count + 1 end
		seen_count = count
	`)
	if err != nil {
		t.Fatalf("script error: %v", err)
	}
	// The empty proxy yields no iteration entries.
	if len(got) != 0 {
		t.Errorf("pairs() over the recording proxy recorded %v; expected nothing "+
			"(gopher-lua pairs is raw and cannot be intercepted here)", got)
	}
	// Document the visible consequence: the script sees zero properties via pairs.
	// (If a future gopher-lua/version honors __pairs, this test will flag the change.)
}

// TestRecordingMarshaller_KeysByEntity proves the core of the provenance model:
// two entities marshaled through the SAME record (as happens when a rule reads
// its trigger plus another entity via rela.get_entity in later PRs) keep their
// touched fields distinct — a field read on one is NOT attributed to the other.
func TestRecordingMarshaller_KeysByEntity(t *testing.T) {
	ls := lua.NewState()
	defer ls.Close()
	m, rec := NewRecordingMarshaller()

	a := &entity.Entity{ID: "TKT-A", Type: "ticket", Properties: map[string]any{"status": "open", "priority": "high"}}
	b := &entity.Entity{ID: "TKT-B", Type: "ticket", Properties: map[string]any{"status": "closed", "priority": "low"}}
	ls.SetGlobal("a", m(ls, a))
	ls.SetGlobal("b", m(ls, b))

	// Read `status` on A, `priority` on B — through one shared record.
	if err := ls.DoString(`local _ = a.properties.status; local _ = b.properties.priority`); err != nil {
		t.Fatalf("script error: %v", err)
	}

	if got := rec.AccessedFor("TKT-A"); !equalStringSets(got, []string{"status"}) {
		t.Errorf("TKT-A recorded = %v, want [status]", got)
	}
	if got := rec.AccessedFor("TKT-B"); !equalStringSets(got, []string{"priority"}) {
		t.Errorf("TKT-B recorded = %v, want [priority]", got)
	}
	if ents := rec.Entities(); !equalStringSets(ents, []string{"TKT-A", "TKT-B"}) {
		t.Errorf("Entities() = %v, want [TKT-A TKT-B]", ents)
	}
	if rec.Has("TKT-A", "priority") {
		t.Errorf("LEAK of attribution: A must not record B's field 'priority'")
	}
}

func equalStringSets(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	m := make(map[string]struct{}, len(a))
	for _, s := range a {
		m[s] = struct{}{}
	}
	for _, s := range b {
		if _, ok := m[s]; !ok {
			return false
		}
	}
	return true
}
