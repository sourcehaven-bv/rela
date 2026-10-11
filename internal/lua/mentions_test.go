package lua_test

import (
	"context"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/lua"
	"github.com/Sourcehaven-BV/rela/internal/principal"
)

// TestScriptReads_MentionedByGated (TKT-Q6AKUN): rela.get_mentioned_by lists
// only the sources the caller may read, redacted, and pages by cursor.
func TestScriptReads_MentionedByGated(t *testing.T) {
	st, deps := newACLWorld(t)
	ctx := context.Background()
	for _, e := range []*entity.Entity{
		{ID: "TKT-2", Type: "ticket", Properties: map[string]any{"title": "Two"}, Content: "Owner `P-1`."},
		{ID: "P-2", Type: "person", Properties: map[string]any{"name": "Bo", "salary": "5"}, Content: "Reports to `P-1`."},
		{ID: "SEC-2", Type: "secret", Properties: map[string]any{"title": "Hidden"}, Content: "About `P-1`."},
	} {
		if err := st.CreateEntity(ctx, e); err != nil {
			t.Fatalf("seed %s: %v", e.ID, err)
		}
	}

	out := runAsAlice(t, deps, `
local page = rela.get_mentioned_by("P-1")
for _, e in ipairs(page.items) do
  rela.output(e.id .. " salary=" .. tostring(e.properties.salary))
end
rela.output("next=" .. tostring(page.next_cursor))
local first = rela.get_mentioned_by("P-1", {limit = 1})
rela.output("first=" .. first.items[1].id .. " cursor=" .. first.next_cursor)
local second = rela.get_mentioned_by("P-1", {limit = 1, cursor = first.next_cursor})
rela.output("second=" .. second.items[1].id)
rela.output("hidden=" .. #rela.get_mentioned_by("SEC-1").items)
`)
	for _, want := range []string{
		"P-2 salary=nil", "TKT-2 salary=nil", "next=nil", "first=P-2 cursor=P-2", "second=TKT-2", "hidden=0",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in output:\n%s", want, out)
		}
	}
	if strings.Contains(out, "SEC-2") {
		t.Errorf("hidden source leaked: %s", out)
	}
}

// TestScriptReads_MentionedByRejectsBadOpts pins that a malformed option
// raises instead of falling back to a default.
func TestScriptReads_MentionedByRejectsBadOpts(t *testing.T) {
	_, deps := newACLWorld(t)
	for _, src := range []string{
		`rela.get_mentioned_by("P-1", {limit = 0})`,
		`rela.get_mentioned_by("P-1", {limit = 201})`,
		`rela.get_mentioned_by("P-1", {limit = 1.5})`,
		`rela.get_mentioned_by("P-1", {limit = "5"})`,
		`rela.get_mentioned_by("P-1", {cursor = 5})`,
		`rela.get_mentioned_by("P-1", {cursor = "not an id"})`,
		`rela.get_mentioned_by("P-1", {cursor = "P-2@draft"})`,
		`rela.get_mentioned_by("")`,
	} {
		t.Run(src, func(t *testing.T) {
			pctx := principal.With(context.Background(), principal.Principal{User: "alice", Tool: principal.ToolScheduler})
			var out strings.Builder
			rt := lua.NewWriter(deps, &out, lua.WithContext(pctx), lua.WithPrincipal(principal.From(pctx)))
			defer rt.Close()
			if err := rt.RunString(src); err == nil || !strings.Contains(err.Error(), "get_mentioned_by") {
				t.Errorf("want a get_mentioned_by error, got %v", err)
			}
		})
	}
}
