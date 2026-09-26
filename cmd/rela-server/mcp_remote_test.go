package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	mcpgo "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/appbuild/appbuildtest"
	"github.com/Sourcehaven-BV/rela/internal/dataentry"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
)

// Fixture for the remote MCP tests. `alice` may read tickets and notes but not
// features. On tickets she may see title and status but not `secret`; on notes
// she may see status only, so a note's title is a hidden field.
const (
	remoteTicketID   = "TKT-001"
	remoteFeatureID  = "FEAT-001"
	remoteNoteID     = "NOTE-001"
	remoteHiddenFeat = "zephyr feature title"
	remoteNoteTitle  = "classified note title"
	remoteSecret     = "sesame"
)

// remoteMCPClient builds the server through the production constructor
// [newRemoteMCPServer], serves it over HTTP with every request stamped as
// alice (standing in for the JWT gate), and returns a connected client.
func remoteMCPClient(t *testing.T) *mcpgo.ClientSession {
	t.Helper()
	ctx := context.Background()

	props := func(names ...string) map[string]metamodel.PropertyDef {
		m := make(map[string]metamodel.PropertyDef, len(names))
		for _, n := range names {
			m[n] = metamodel.PropertyDef{Type: "string"}
		}
		return m
	}
	meta := &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"ticket":  {Label: "Ticket", IDPrefix: "TKT", Properties: props("title", "status", "secret")},
			"feature": {Label: "Feature", IDPrefix: "FEAT", Properties: props("title")},
			"note":    {Label: "Note", IDPrefix: "NOTE", Properties: props("title", "status")},
		},
	}

	st := memstore.New()
	ticket := entity.New(remoteTicketID, "ticket")
	ticket.SetString("title", "alpha visible ticket")
	ticket.SetString("status", "open")
	ticket.SetString("secret", remoteSecret)
	feature := entity.New(remoteFeatureID, "feature")
	feature.SetString("title", "alpha "+remoteHiddenFeat)
	note := entity.New(remoteNoteID, "note")
	note.SetString("title", remoteNoteTitle)
	note.SetString("status", "draft")
	note.Content = "alpha body"
	for _, e := range []*entity.Entity{ticket, feature, note} {
		if err := st.CreateEntity(ctx, e); err != nil {
			t.Fatalf("seed %s: %v", e.ID, err)
		}
	}

	d, err := acl.NewDeclarative(&acl.Policy{
		Roles: map[string]acl.RoleDef{"viewer": {
			Read: []string{"ticket", "note"},
			Visible: map[string][]acl.FieldGrant{
				"ticket": {{Field: "title"}, {Field: "status"}},
				"note":   {{Field: "status"}},
			},
		}},
		Assignments: map[string]string{"alice": "viewer"},
	}, acl.NewStoreGraph(st), st)
	if err != nil {
		t.Fatalf("acl.NewDeclarative: %v", err)
	}

	svc := appbuildtest.New(meta, appbuildtest.WithStore(st), appbuildtest.WithDeclarative(d))
	t.Cleanup(func() { _ = svc.Close() })

	host := dataentry.MCPHost{
		AttachmentPolicy: func() (*metamodel.Metamodel, int64) { return meta, 0 },
		WriteLock:        &sync.Mutex{},
	}
	srv, err := newRemoteMCPServer(svc, host)
	if err != nil {
		t.Fatalf("newRemoteMCPServer: %v", err)
	}
	alice := principal.Principal{User: "alice", Tool: principal.ToolMCP}
	h := srv.HTTPHandler()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h.ServeHTTP(w, r.WithContext(principal.With(r.Context(), alice)))
	}))
	t.Cleanup(ts.Close)

	client := mcpgo.NewClient(&mcpgo.Implementation{Name: "test", Version: "0"}, nil)
	cs, err := client.Connect(ctx, &mcpgo.StreamableClientTransport{
		Endpoint: ts.URL, DisableStandaloneSSE: true,
	}, nil)
	if err != nil {
		t.Fatalf("client connect: %v", err)
	}
	t.Cleanup(func() { _ = cs.Close() })
	return cs
}

// searchRemote calls search_entities and decodes the summaries.
func searchRemote(t *testing.T, cs *mcpgo.ClientSession, query string) (raw string, hits []map[string]any) {
	t.Helper()
	res, err := cs.CallTool(context.Background(), &mcpgo.CallToolParams{
		Name: "search_entities", Arguments: map[string]any{"query": query},
	})
	if err != nil {
		t.Fatalf("search_entities(%q): %v", query, err)
	}
	if res.IsError || len(res.Content) == 0 {
		t.Fatalf("search_entities(%q): error result %+v", query, res.Content)
	}
	tc, ok := res.Content[0].(*mcpgo.TextContent)
	if !ok {
		t.Fatalf("search_entities(%q): content is %T, want text", query, res.Content[0])
	}
	if err := json.Unmarshal([]byte(tc.Text), &hits); err != nil {
		t.Fatalf("decode %q: %v", tc.Text, err)
	}
	return tc.Text, hits
}

// TestRemoteMCP_NoLuaTools pins TKT-UIR41P AC 7. The Lua runtime reads
// through unrestricted deps, so a remote lua_eval would bypass the row gate
// and `visible:` redaction. The tools must be absent from tools/list and a
// call must fail. The stdio server still lists them; that side is pinned by
// internal/mcp's TestDispatch_ToolInventoryMatches.
func TestRemoteMCP_NoLuaTools(t *testing.T) {
	t.Parallel()
	cs := remoteMCPClient(t)
	ctx := context.Background()

	list, err := cs.ListTools(ctx, &mcpgo.ListToolsParams{})
	if err != nil {
		t.Fatalf("tools/list: %v", err)
	}
	if len(list.Tools) == 0 {
		t.Fatal("tools/list is empty; the fixture is broken")
	}
	for _, tool := range list.Tools {
		if strings.HasPrefix(tool.Name, "lua_") {
			t.Errorf("remote tools/list contains %q; Lua tools are stdio-only", tool.Name)
		}
	}

	for _, call := range []struct {
		name string
		args map[string]any
	}{
		{"lua_eval", map[string]any{"code": `return rela.get_entity("` + remoteFeatureID + `")`}},
		{"lua_run", map[string]any{"path": "x.lua"}},
		{"lua_list", map[string]any{}},
	} {
		res, err := cs.CallTool(ctx, &mcpgo.CallToolParams{Name: call.name, Arguments: call.args})
		if err == nil && !res.IsError {
			t.Errorf("%s succeeded remotely; want it rejected as an unknown tool", call.name)
		}
	}
}

// TestRemoteMCP_SearchExcludesUnreadable pins that search_entities returns
// only rows the caller may read, and neither counts nor names a hidden one.
func TestRemoteMCP_SearchExcludesUnreadable(t *testing.T) {
	t.Parallel()
	cs := remoteMCPClient(t)

	raw, hits := searchRemote(t, cs, "alpha")
	ids := make(map[string]bool, len(hits))
	for _, h := range hits {
		ids[h["id"].(string)] = true
	}
	if !ids[remoteTicketID] {
		t.Errorf("readable ticket missing from results: %s", raw)
	}
	if ids[remoteFeatureID] || strings.Contains(raw, remoteHiddenFeat) {
		t.Errorf("unreadable feature leaked into search results: %s", raw)
	}
	if len(hits) != len(ids) || len(hits) != 2 {
		t.Errorf("got %d hits, want exactly the ticket and the note: %s", len(hits), raw)
	}
}

// TestRemoteMCP_SearchRedactsHiddenTitle pins that a hit on a readable row
// whose title is a `visible:`-hidden field carries no title.
func TestRemoteMCP_SearchRedactsHiddenTitle(t *testing.T) {
	t.Parallel()
	cs := remoteMCPClient(t)

	raw, hits := searchRemote(t, cs, "alpha")
	var note map[string]any
	for _, h := range hits {
		if h["id"] == remoteNoteID {
			note = h
		}
	}
	if note == nil {
		t.Fatalf("note matched on its content but is missing: %s", raw)
	}
	if _, has := note["title"]; has || strings.Contains(raw, remoteNoteTitle) {
		t.Errorf("hidden note title reached the result: %s", raw)
	}
	if note["status"] != "draft" {
		t.Errorf("note status = %v, want the visible value \"draft\"", note["status"])
	}
}

// TestRemoteMCP_SearchDropsHiddenFieldMatch pins the match-on-hidden-field
// oracle (TKT-GGQ0JT) on this surface: a query that matches a readable row
// only through a hidden property must not return that row.
func TestRemoteMCP_SearchDropsHiddenFieldMatch(t *testing.T) {
	t.Parallel()
	cs := remoteMCPClient(t)

	for _, q := range []string{remoteSecret, "classified"} {
		raw, hits := searchRemote(t, cs, q)
		if len(hits) != 0 {
			t.Errorf("search %q matched only hidden fields but returned %s", q, raw)
		}
	}
}
