package dataentry

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	v1 "github.com/Sourcehaven-BV/rela/internal/apiwire/v1"
	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	"github.com/Sourcehaven-BV/rela/internal/entity"
)

// seedSidebarWorld seeds 10 tickets — 5 visible to alice (belongs-to
// PRJ-42, which she is editor-of), 5 hidden (belongs-to PRJ-9). Of the
// visible 5, 3 are status=open; of the hidden 5, 2 are status=open.
func seedSidebarWorld(app *App) {
	seedEntity(app, &entity.Entity{ID: "alice", Type: "person", Properties: map[string]any{"title": "Alice"}})
	seedEntity(app, &entity.Entity{ID: "PRJ-42", Type: "project", Properties: map[string]any{"title": "Granted"}})
	seedEntity(app, &entity.Entity{ID: "PRJ-9", Type: "project", Properties: map[string]any{"title": "Hidden"}})
	seedRelation(app, entity.NewRelation("alice", "editor-of", "PRJ-42"))
	for i := 1; i <= 5; i++ {
		status := "closed"
		if i <= 3 {
			status = "open"
		}
		id := fmt.Sprintf("TKT-V%02d", i)
		seedEntity(app, &entity.Entity{ID: id, Type: "ticket",
			Properties: map[string]any{"title": id, "status": status}})
		seedRelation(app, entity.NewRelation(id, "belongs-to", "PRJ-42"))
	}
	for i := 1; i <= 5; i++ {
		status := "closed"
		if i <= 2 {
			status = "open"
		}
		id := fmt.Sprintf("TKT-H%02d", i)
		seedEntity(app, &entity.Entity{ID: id, Type: "ticket",
			Properties: map[string]any{"title": id, "status": status}})
		seedRelation(app, entity.NewRelation(id, "belongs-to", "PRJ-9"))
	}
}

// sidebarPolicy grants read on ticket via editor-of + belongs-to
// containment — the same shape the list tests use, so sidebar and
// list verdicts are directly comparable.
func sidebarPolicy() *acl.Policy {
	return &acl.Policy{
		Roles: map[string]acl.RoleDef{"editor": {Read: []string{"ticket"}}},
		RoleRelations: map[string]acl.RoleRelationDef{
			"editor-of": {Confers: "editor"},
		},
		InheritRolesThrough: []string{"belongs-to"},
	}
}

// installSidebarConfig publishes a Config snapshot carrying one plain
// list, one filtered list, and one kanban over tickets.
func installSidebarConfig(app *App) {
	app.mutateState(func(s *AppState) {
		cfg := *s.Cfg
		cfg.Lists = map[string]dataentryconfig.List{
			"all-tickets": {EntityType: "ticket", Title: "All"},
			"open-tickets": {EntityType: "ticket", Title: "Open",
				Filters: []dataentryconfig.FilterConfig{{Property: "status", Operator: "=", Value: "open"}}},
		}
		cfg.Kanbans = map[string]dataentryconfig.Kanban{
			"board": {EntityType: "ticket", ColumnProperty: "status"},
		}
		cfg.Navigation = []dataentryconfig.NavigationEntry{
			{Label: "All", List: "all-tickets"},
			{Label: "Open", List: "open-tickets"},
			{Label: "Board", Kanban: "board"},
		}
		s.Cfg = &cfg
	})
}

// sidebarCountsByLabel performs a sidebar request and returns the
// label → count map for top-level items.
func sidebarCountsByLabel(ctx context.Context, t *testing.T, app *App) map[string]int {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/_sidebar", http.NoBody)
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()
	app.handleV1Sidebar(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET _sidebar: %d %s", rec.Code, rec.Body)
	}
	var resp v1.SidebarResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode sidebar: %v\nbody: %s", err, rec.Body)
	}
	out := make(map[string]int)
	for _, group := range resp.Navigation {
		for _, item := range group.Items {
			if item.Count != nil {
				out[item.Label] = *item.Count
			}
		}
	}
	return out
}

// TestACLSidebar_CountsMatchList pins TKT-VMD8 AC6: sidebar list and
// kanban counts reflect the principal's visible subset (5 of 10), so
// the sidebar can never disagree with the list it links to.
func TestACLSidebar_CountsMatchList(t *testing.T) {
	app := newTestAppV1(t)
	seedSidebarWorld(app)
	installSidebarConfig(app)

	d := mustNewACL(t, sidebarPolicy(), app.store)
	app.acl = d

	counts := sidebarCountsByLabel(gateCtxFor(aliceCtx(), t, d), t, app)
	if counts["All"] != 5 {
		t.Errorf("list count = %d, want 5 (visible subset of 10)", counts["All"])
	}
	if counts["Board"] != 5 {
		t.Errorf("kanban count = %d, want 5 (visible subset of 10)", counts["Board"])
	}
}

// TestACLSidebar_ConfigFilterIntersection pins TKT-VMD8 AC7: a
// config-filtered list counts the intersection visible ∩ filter —
// ACL runs first, the config filter applies in-memory after.
func TestACLSidebar_ConfigFilterIntersection(t *testing.T) {
	app := newTestAppV1(t)
	seedSidebarWorld(app)
	installSidebarConfig(app)

	d := mustNewACL(t, sidebarPolicy(), app.store)
	app.acl = d

	counts := sidebarCountsByLabel(gateCtxFor(aliceCtx(), t, d), t, app)
	// 5 visible, 3 of them open. The 2 hidden-but-open tickets must
	// not count.
	if counts["Open"] != 3 {
		t.Errorf("filtered list count = %d, want 3 (visible ∩ open)", counts["Open"])
	}
}

// TestACLSidebar_DenyAllZeroCounts: a principal with no grant sees
// zero counts everywhere — cardinality of hidden types does not leak
// through the sidebar.
func TestACLSidebar_DenyAllZeroCounts(t *testing.T) {
	app := newTestAppV1(t)
	seedSidebarWorld(app)
	installSidebarConfig(app)

	d := mustNewACL(t, sidebarPolicy(), app.store)
	app.acl = d

	counts := sidebarCountsByLabel(gateCtxFor(principalCtx("mallory"), t, d), t, app)
	for _, label := range []string{"All", "Open", "Board"} {
		if counts[label] != 0 {
			t.Errorf("%s count = %d for deny-all principal, want 0", label, counts[label])
		}
	}
}

// TestACLSidebar_NopACLFullCounts pins the single-mode invariant
// (RR-2O27): without ACL the same code path yields the full counts —
// there is no separate "ACL off" branch to drift.
func TestACLSidebar_NopACLFullCounts(t *testing.T) {
	app := newTestAppV1(t)
	seedSidebarWorld(app)
	installSidebarConfig(app)

	// No gate on ctx → nopReadGate → AllowAll.
	counts := sidebarCountsByLabel(context.Background(), t, app)
	if counts["All"] != 10 {
		t.Errorf("list count = %d, want 10", counts["All"])
	}
	if counts["Open"] != 5 {
		t.Errorf("filtered list count = %d, want 5 (all open)", counts["Open"])
	}
	if counts["Board"] != 10 {
		t.Errorf("kanban count = %d, want 10", counts["Board"])
	}
}
