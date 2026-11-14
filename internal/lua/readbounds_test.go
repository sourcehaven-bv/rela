package lua_test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/lua"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
	"github.com/Sourcehaven-BV/rela/internal/store/storetest"
	"github.com/Sourcehaven-BV/rela/internal/visibility"
)

// TKT-P9QMEVI: rela.get_relations, admin.get_relations, admin.list_entities
// and rela.md.entity_refs read a bounded amount, whatever the store holds.

// paddedACLWorld is newACLWorld over a counting memstore with n extra
// persons, each owned by TKT-1: rows alice may read, of the type and
// relation type the bindings under test read. The counter is reset after
// seeding.
func paddedACLWorld(t *testing.T, n int) (*storetest.Counting, lua.WriteDeps, *acl.Declarative) {
	t.Helper()
	counting := storetest.NewCounting(memstore.New())
	_, deps, decl := newACLWorldOn(t, counting)
	ctx := context.Background()
	for i := range n {
		id := fmt.Sprintf("P-PAD%04d", i)
		if err := counting.CreateEntity(ctx, &entity.Entity{
			ID: id, Type: "person", Properties: map[string]any{"name": id},
		}); err != nil {
			t.Fatalf("pad %s: %v", id, err)
		}
		if _, err := counting.CreateRelation(ctx, entity.RelationKey{From: "TKT-1", Type: "owns", To: id}, nil); err != nil {
			t.Fatalf("pad edge %s: %v", id, err)
		}
	}
	deps.ElevatedReader = visibility.Unrestricted(counting).WithWorld(visibility.WorldOf(store.TrivialScope()))
	counting.Reset()
	return counting, deps, decl
}

// TestReadBindings_CostIsScaleFree pins that each binding's store cost
// follows its limit or its ids, not the store. Before TKT-P9QMEVI every one
// of them read every row the padding adds.
func TestReadBindings_CostIsScaleFree(t *testing.T) {
	for _, tc := range []struct{ name, src string }{
		{"get_relations", `rela.output(#rela.get_relations({type = "owns", limit = 5}))`},
		{"get_relations from", `rela.output(#rela.get_relations({from = "TKT-1", limit = 5}))`},
		{"admin.get_relations", `rela.bypass_acl(function(admin)
			rela.output(#admin.get_relations({type = "owns", limit = 5})) end)`},
		{"admin.list_entities", `rela.bypass_acl(function(admin)
			rela.output(#admin.list_entities("person", {limit = 5})) end)`},
		{"entity_refs content", `local n = 0
			for _ in pairs(rela.md.entity_refs({content = "see ` + "`P-1` and `TKT-1`" + `"})) do n = n + 1 end
			rela.output(n)`},
		{"entity_refs ids", `local n = 0
			for _ in pairs(rela.md.entity_refs({ids = {"P-1", "SEC-1"}})) do n = n + 1 end
			rela.output(n)`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var outs []string
			storetest.AssertScaleFree(t, func(n int) (storetest.Cost, string) {
				counting, deps, decl := paddedACLWorld(t, n)
				outs = append(outs, runAsAlice(t, decl, deps, tc.src))
				return counting.Cost(), counting.String()
			})
			if outs[0] != outs[1] {
				t.Errorf("output depends on the store size: %q vs %q", outs[0], outs[1])
			}
		})
	}
}

// TestGetRelations_LimitAndDefault pins the limit option, its clamp to the
// ceiling, and that no limit still returns every edge under the ceiling.
func TestGetRelations_LimitAndDefault(t *testing.T) {
	_, deps, decl := paddedACLWorld(t, 30)
	for _, tc := range []struct{ src, want string }{
		{`rela.output(#rela.get_relations({type = "owns", limit = 7}))`, "7"},
		// 30 padded edges plus TKT-1 owns P-1; TKT-1 owns SEC-1 is peer-gated.
		{`rela.output(#rela.get_relations({type = "owns"}))`, "31"},
		{`rela.output(#rela.get_relations({type = "owns", limit = 999999}))`, "31"},
		{`rela.bypass_acl(function(admin) rela.output(#admin.get_relations({type = "owns", limit = 4})) end)`, "4"},
		{`rela.bypass_acl(function(admin) rela.output(#admin.get_relations({type = "owns"})) end)`, "32"},
		{`rela.bypass_acl(function(admin) rela.output(#admin.list_entities("person", {limit = 3})) end)`, "3"},
		{`rela.bypass_acl(function(admin) rela.output(#admin.list_entities("person")) end)`, "31"},
	} {
		if got := strings.TrimSpace(runAsAlice(t, decl, deps, tc.src)); got != tc.want {
			t.Errorf("%s = %q, want %q", tc.src, got, tc.want)
		}
	}
}

// TestGetRelations_GatedPageFillsPastHiddenEdges pins that a hidden edge
// inside the first raw page does not shorten the page: the reader reads on
// and serves the next visible edge.
func TestGetRelations_GatedPageFillsPastHiddenEdges(t *testing.T) {
	st, deps, decl := newACLWorld(t)
	ctx := context.Background()
	// P-2 is a secret alice may not read; its edge sorts between P-1's and P-3's.
	for _, e := range []*entity.Entity{
		{ID: "P-2", Type: "secret", Properties: map[string]any{"title": "x"}},
		{ID: "P-3", Type: "person", Properties: map[string]any{"name": "Cy"}},
	} {
		if err := st.CreateEntity(ctx, e); err != nil {
			t.Fatalf("seed %s: %v", e.ID, err)
		}
		if _, err := st.CreateRelation(ctx, entity.RelationKey{From: "TKT-1", Type: "owns", To: e.ID}, nil); err != nil {
			t.Fatalf("seed edge: %v", err)
		}
	}
	out := runAsAlice(t, decl, deps, `
local ids = {}
for _, r in ipairs(rela.get_relations({from = "TKT-1", limit = 2})) do ids[#ids+1] = r.to end
rela.output(table.concat(ids, ","))`)
	if got := outputString(out); got != "P-1,P-3" {
		t.Errorf("relations = %q, want P-1,P-3: the hidden P-2 edge must neither show nor shorten the page", got)
	}
}

// TestGetRelations_RaisesOnStoreError is the TKT-FVQ4 regression test for
// rela.get_relations, which used to return a short list on a store error.
func TestGetRelations_RaisesOnStoreError(t *testing.T) {
	t.Parallel()
	_, deps, _ := newACLWorld(t)
	deps.VisibleReader = visibility.DenyReader{}
	var out strings.Builder
	rt := lua.NewWriter(deps, &out)
	defer rt.Close()
	err := rt.RunString(`rela.get_relations({type = "owns"})`)
	if err == nil || !strings.Contains(err.Error(), "rela.get_relations") {
		t.Fatalf("err = %v, want the read failure raised by rela.get_relations", err)
	}
}

// TestReadBindings_RejectBadLimit pins the shared limit validation.
func TestReadBindings_RejectBadLimit(t *testing.T) {
	t.Parallel()
	_, deps, _ := paddedACLWorld(t, 0)
	for _, src := range []string{
		`rela.get_relations({limit = 0})`,
		`rela.get_relations({limit = 1.5})`,
		`rela.get_relations({limit = "5"})`,
		`rela.bypass_acl(function(admin) admin.get_relations({limit = -1}) end)`,
		`rela.bypass_acl(function(admin) admin.list_entities("person", {limit = 0}) end)`,
		`rela.bypass_acl(function(admin) admin.list_entities("person", {limt = 5}) end)`,
		`rela.md.entity_refs({limit = 0})`,
	} {
		var out strings.Builder
		rt := lua.NewWriter(deps, &out)
		if err := rt.RunString(src); err == nil {
			t.Errorf("%s was accepted", src)
		}
		rt.Close()
	}
}

// TestEntityRefs_IDs pins the id forms of rela.md.entity_refs: only the
// named ids, gated, filtered by types, and refused past the limit.
func TestEntityRefs_IDs(t *testing.T) {
	_, deps, decl := newACLWorld(t)
	keys := `local ks = {}
for id in pairs(refs) do ks[#ks+1] = id end
table.sort(ks)
rela.output(table.concat(ks, ","))`
	for _, tc := range []struct{ name, call, want string }{
		{"ids", `rela.md.entity_refs({ids = {"TKT-1", "SEC-1", "NOPE-1", "TKT-1"}})`, "TKT-1"},
		{"content", "rela.md.entity_refs({content = \"`P-1` and `SEC-1`, not P-9 or\\n\\n```\\n`TKT-1`\\n```\"})", "P-1"},
		{"types filter", `rela.md.entity_refs({ids = {"TKT-1", "P-1"}, types = {"person"}})`, "P-1"},
		{"empty ids", `rela.md.entity_refs({ids = {}})`, ""},
		{"legacy limit", `rela.md.entity_refs({types = {"ticket", "person"}, limit = 1})`, "TKT-1"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			out := runAsAlice(t, decl, deps, "local refs = "+tc.call+"\n"+keys)
			if got := outputString(out); got != tc.want {
				t.Errorf("refs = %q, want %q", got, tc.want)
			}
		})
	}

	for _, src := range []string{
		`rela.md.entity_refs({ids = {"TKT-1"}, content = "x"})`,
		`rela.md.entity_refs({ids = {"TKT-1", "TKT-2", "TKT-3"}, limit = 2})`,
		`rela.md.entity_refs({ids = {"TKT-1@draft"}})`,
		`rela.md.entity_refs({contents = "x"})`,
		`rela.md.entity_refs({ids = {""}})`,
		`rela.md.entity_refs({ids = "TKT-1"})`,
		`rela.md.entity_refs({content = 5})`,
	} {
		var out strings.Builder
		rt := lua.NewWriter(deps, &out)
		if err := rt.RunString(src); err == nil {
			t.Errorf("%s was accepted", src)
		}
		rt.Close()
	}
}

// TestEntityRefs_LegacyRaisesPastDefaultLimit pins that entity_refs()
// without ids, content or limit raises instead of returning a cut map once
// the readable entities exceed the ceiling. Not parallel: it lowers the
// package ceiling.
func TestEntityRefs_LegacyRaisesPastDefaultLimit(t *testing.T) {
	_, deps, decl := newACLWorld(t)
	lua.SwapMaxReadLimit(t, 1) // alice reads TKT-1 and P-1
	err := runAsAliceErr(t, decl, deps, `rela.md.entity_refs({types = {"ticket", "person"}})`)
	if err == nil || !strings.Contains(err.Error(), "pass ids or content") {
		t.Errorf("err = %v, want the cut map refused", err)
	}
	out := runAsAlice(t, decl, deps, `local refs = rela.md.entity_refs({types = {"ticket", "person"}, limit = 1})
local n = 0
for _ in pairs(refs) do n = n + 1 end
rela.output(n)`)
	if got := outputString(out); got != "1" {
		t.Errorf("explicit limit: %s refs, want 1", got)
	}
}

// outputString strips the JSON quotes rela.output puts around a string.
func outputString(out string) string {
	return strings.Trim(strings.TrimSpace(out), `"`)
}
