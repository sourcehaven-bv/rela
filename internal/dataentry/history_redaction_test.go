package dataentry

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	v1 "github.com/Sourcehaven-BV/rela/internal/apiwire/v1"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/search"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// permGate is a readGate that grants a configurable SET of named permissions
// (unlike fakeGate's single bool), so the two history permissions
// (history:read and history:read-redacted) can be granted independently. Reads
// are permitted (the live entity path), matching an authorized reader.
type permGate struct {
	perms map[string]bool
}

func (g permGate) PermitsRead(context.Context, string, string) (bool, error) { return true, nil }

func (g permGate) PermitsReadMany(_ context.Context, _ string, ids []string) (map[string]bool, error) {
	m := make(map[string]bool, len(ids))
	for _, id := range ids {
		m[id] = true
	}
	return m, nil
}

func (g permGate) ReadQuery(context.Context, string) acl.ReadQueryResult {
	return acl.ReadQueryResult{AllowAll: true}
}

func (g permGate) SearchScope(context.Context, []string) map[string]search.TypeScope {
	return map[string]search.TypeScope{search.WildcardType: {AllowAll: true}}
}

func (g permGate) HoldsPermission(_ context.Context, perm string) bool { return g.perms[perm] }

// getHistoryVersion drives serveHistoryVersion via handleV1History for version 1
// of id, under the given principal and read gate.
func getHistoryVersion(app *App, user, typeName, id string, gate readGate) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/_history/"+typeName+"/"+id+"/1", http.NoBody)
	ctx := principal.With(req.Context(),
		principal.Principal{User: user, Tool: principal.ToolDataEntry})
	ctx = withReadGate(ctx, gate)
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()
	handleV1History(app, rec, req)
	return rec
}

func decodeHistoryEntity(t *testing.T, rec *httptest.ResponseRecorder) v1.Entity {
	t.Helper()
	if rec.Code != http.StatusOK {
		t.Fatalf("history version: got %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	var resp struct {
		Entity v1.Entity `json:"entity"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode history version: %v", err)
	}
	return resp.Entity
}

// TKT-73C6B2 — the conditional-visible policy shared by the redaction tests:
// `priority` is visible only when the entity has an outgoing `depends_on` edge (a
// subject-world condition). Live, with the edge present, it shows; in history
// the marker neuters the lookup so it fails closed.
const historyRedactionACL = `
roles:
  triager:
    read:
      - ticket
    visible:
      ticket:
        - field: title
        - field: status
        - field: priority
          when: "has_relation(entity, 'depends_on')"
assignments:
  alice: triager
`

// Scenario 1/2 (fail closed): a has_relation-gated visible field that is shown
// on the LIVE entity (edge present) is HIDDEN in the historical snapshot — the
// marker neuters the subject-world lookup so the grant can't be affirmed. This
// holds even though the live store STILL has the edge, which is exactly the
// leak the design prevents (a deleted/drifted entity would have no edge, and we
// must not trust the live store either way).
func TestHistoryRedaction_SubjectConditional_FailsClosed(t *testing.T) {
	app := buildPolicyApp(t, historyRedactionACL, nil)
	seedEntity(app, &entity.Entity{
		ID:         "TKT-001",
		Type:       "ticket",
		Properties: map[string]any{"title": "t", "priority": "high", "status": "open"},
	})
	// Live edge present → the grant passes on a LIVE read.
	if _, err := app.store.CreateRelation(context.Background(),
		"TKT-001", "depends_on", "TKT-002", nil); err != nil {
		t.Fatalf("CreateRelation: %v", err)
	}
	app.versions = historyStore{
		versions: map[string][]store.VersionSnapshot{
			"TKT-001": {snapshot("ticket", "body",
				map[string]any{"title": "t", "priority": "high", "status": "open"})},
		},
	}

	// Sanity: LIVE read shows priority (edge present, grant passes).
	live := getAs(t, app, "alice", "TKT-001")
	if _, ok := live.Properties["priority"]; !ok {
		t.Fatalf("live read should expose priority (depends_on edge present); props=%v", live.Properties)
	}

	// Historical read (no reveal permission): priority must be stripped.
	gate := permGate{perms: map[string]bool{acl.PermHistoryRead: true}}
	hist := decodeHistoryEntity(t, getHistoryVersion(app, "alice", "ticket", "TKT-001", gate))
	if _, ok := hist.Properties["priority"]; ok {
		t.Errorf("historical read must fail closed: priority should be stripped, got %v", hist.Properties["priority"])
	}
	// Non-conditional fields remain.
	if _, ok := hist.Properties["title"]; !ok {
		t.Errorf("historical read stripped a non-hidden field (title); props=%v", hist.Properties)
	}
}

// C1 regression (RR — the role-resolution leak): a `visible:` grant on a role
// conferred by a LIVE role-relation edge (`role_relations`) must ALSO fail
// closed in history. The subject-world neutering has to reach role resolution,
// not just has_relation/count_relations — otherwise a role newly conferred
// after capture (a reassignment) selects a `visible:` block that reveals a
// field hidden when the version was written. Globals-only under the historical
// marker is the fix.
func TestHistoryRedaction_LocalRoleConferred_FailsClosed(t *testing.T) {
	const aclYAML = `
roles:
  owner:
    read:
      - ticket
    visible:
      ticket:
        - field: title
        - field: status
        - field: priority
role_relations:
  owns:
    confers: owner
`
	app := buildPolicyApp(t, aclYAML, nil)
	seedEntity(app, &entity.Entity{
		ID:         "TKT-001",
		Type:       "ticket",
		Properties: map[string]any{"title": "t", "priority": "high", "status": "open"},
	})
	// Live edge alice --owns--> TKT-001 confers `owner` on alice for this entity.
	if _, err := app.store.CreateRelation(context.Background(),
		"alice", "owns", "TKT-001", nil); err != nil {
		t.Fatalf("CreateRelation: %v", err)
	}
	app.versions = historyStore{
		versions: map[string][]store.VersionSnapshot{
			"TKT-001": {snapshot("ticket", "body",
				map[string]any{"title": "t", "priority": "high", "status": "open"})},
		},
	}

	// Sanity: LIVE read confers owner via the edge → priority visible.
	live := getAs(t, app, "alice", "TKT-001")
	if _, ok := live.Properties["priority"]; !ok {
		t.Fatalf("live read should confer owner and expose priority; props=%v", live.Properties)
	}

	// Historical read (no reveal permission): the owner role is conferred by a
	// LIVE edge, but historical resolution is globals-only → alice holds no
	// role that grants `priority` → it must be stripped. Before the C1 fix this
	// leaked (owner selected against the live edge).
	gate := permGate{perms: map[string]bool{acl.PermHistoryRead: true}}
	hist := decodeHistoryEntity(t, getHistoryVersion(app, "alice", "ticket", "TKT-001", gate))
	if _, ok := hist.Properties["priority"]; ok {
		t.Errorf("historical read must fail closed on local-role-conferred visibility: priority should be stripped, got %v",
			hist.Properties["priority"])
	}
}

// Scenario 2/4 (reveal): a holder of history:read-redacted sees the frozen
// field the ordinary reader has redacted — OVERRIDE semantics.
func TestHistoryRedaction_RevealPermission_ShowsFrozenField(t *testing.T) {
	app := buildPolicyApp(t, historyRedactionACL, nil)
	seedEntity(app, &entity.Entity{
		ID:         "TKT-001",
		Type:       "ticket",
		Properties: map[string]any{"title": "t", "priority": "high", "status": "open"},
	})
	app.versions = historyStore{
		versions: map[string][]store.VersionSnapshot{
			"TKT-001": {snapshot("ticket", "body",
				map[string]any{"title": "t", "priority": "high", "status": "open"})},
		},
	}

	// With history:read-redacted the marker is skipped → priority revealed.
	gate := permGate{perms: map[string]bool{
		acl.PermHistoryRead:         true,
		acl.PermHistoryReadRedacted: true,
	}}
	hist := decodeHistoryEntity(t, getHistoryVersion(app, "alice", "ticket", "TKT-001", gate))
	if got, ok := hist.Properties["priority"]; !ok || got != "high" {
		t.Errorf("reveal: priority should be present=high, got ok=%v v=%v", ok, got)
	}
}
