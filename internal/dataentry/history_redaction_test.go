package dataentry

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	v1 "github.com/Sourcehaven-BV/rela/internal/apiwire/v1"
	"github.com/Sourcehaven-BV/rela/internal/audit"
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

func (permGate) PermitsWorld(context.Context, string) (bool, error) { return true, nil }

// getHistoryVersion drives serveHistoryVersion via handleV1History for version 1
// of id, under the given principal and read gate.
//
// The entity type is fixed to historyEntityType: every scenario in this file is
// expressed against the one type historyRedactionACL grants on, so a type
// parameter would only ever receive that value.
func getHistoryVersion(app *App, user, id string, gate readGate) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet,
		"/api/v1/_history/"+historyEntityType+"/"+id+"/1", http.NoBody)
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

// historyEntityType is the one entity type historyRedactionACL grants on, and
// so the type every scenario in this file is expressed against.
const historyEntityType = "ticket"

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
  dana: triager
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
	hist := decodeHistoryEntity(t, getHistoryVersion(app, "alice", "TKT-001", gate))
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
	hist := decodeHistoryEntity(t, getHistoryVersion(app, "alice", "TKT-001", gate))
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
	hist := decodeHistoryEntity(t, getHistoryVersion(app, "alice", "TKT-001", gate))
	if got, ok := hist.Properties["priority"]; !ok || got != "high" {
		t.Errorf("reveal: priority should be present=high, got ok=%v v=%v", ok, got)
	}
}

// --- Audit of the reveal (TKT-LVSPSB / issue #1238) ---
//
// The permission exists so a small, deliberately granted group can see frozen
// values hidden from ordinary readers. internal/audit otherwise logs only
// writes, so before this the question "who saw which hidden historical values,
// and when" had no answer.
//
// Two properties matter, and BOTH need a test: the record must appear on a
// reveal, and must NOT appear on an ordinary read. A record that shows up for
// every history read is as useless as one that never shows up, because it
// buries the privileged reads it exists to surface.

// revealAuditFixture seeds the shared reveal scenario and returns the app plus
// its audit sink.
func revealAuditFixture(t *testing.T) (*App, *audit.Memory) {
	t.Helper()
	sink := &audit.Memory{}
	app := buildPolicyApp(t, historyRedactionACL, sink)
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
	return app, sink
}

func TestHistoryReveal_Audited(t *testing.T) {
	app, sink := revealAuditFixture(t)
	gate := permGate{perms: map[string]bool{
		acl.PermHistoryRead:         true,
		acl.PermHistoryReadRedacted: true,
	}}

	// Assert the reveal actually happened, so a green audit assertion below
	// cannot be produced by a request that revealed nothing.
	// Read as dana rather than alice (the user every other test in this file
	// uses), so the principal assertion below cannot pass by coincidence.
	const auditor = "dana"
	hist := decodeHistoryEntity(t, getHistoryVersion(app, auditor, "TKT-001", gate))
	if got := hist.Properties["priority"]; got != "high" {
		t.Fatalf("precondition: expected the reveal to expose priority=high, got %v", got)
	}

	recs := sink.Records()
	if len(recs) != 1 {
		t.Fatalf("reveal must emit exactly one audit record, got %d: %+v", len(recs), recs)
	}
	rec := recs[0]
	if rec.Op != audit.OpHistoryReveal {
		t.Errorf("op = %q, want %q — a reveal must be isolable by op alone",
			rec.Op, audit.OpHistoryReveal)
	}
	if rec.Subject == nil {
		t.Fatal("subject is nil: the record must name WHAT was revealed")
	}
	if rec.Subject.Kind != "entity" || rec.Subject.Type != historyEntityType || rec.Subject.ID != "TKT-001" {
		t.Errorf("subject = %+v, want entity/%s/TKT-001", *rec.Subject, historyEntityType)
	}
	if rec.Principal.User != auditor {
		t.Errorf("principal.user = %q, want %q — the record must name WHO revealed",
			rec.Principal.User, auditor)
	}
	// The version pins WHICH snapshot was disclosed; without it the record
	// cannot distinguish reading one historical value from reading every one.
	if !strings.Contains(rec.Summary, "version=1") {
		t.Errorf("summary = %q, want it to carry version=1", rec.Summary)
	}
	// The revealed VALUE must never reach the audit log: recording it would be
	// a wider disclosure than the read being recorded.
	if strings.Contains(rec.Summary, "high") {
		t.Errorf("summary = %q leaks the revealed value", rec.Summary)
	}
}

// The discriminating case. An ordinary reader gets the redacted view, which
// discloses nothing the permission governs, so it must produce no record.
// Without this test the suite would pass on an implementation that audits every
// history read — which is the failure mode that makes an audit log unusable.
func TestHistoryReveal_OrdinaryReadNotAudited(t *testing.T) {
	app, sink := revealAuditFixture(t)
	gate := permGate{perms: map[string]bool{acl.PermHistoryRead: true}}

	// Precondition: this really is the redacted arm.
	hist := decodeHistoryEntity(t, getHistoryVersion(app, "alice", "TKT-001", gate))
	if _, present := hist.Properties["priority"]; present {
		t.Fatalf("precondition: ordinary reader should have priority redacted, got %v",
			hist.Properties["priority"])
	}

	if recs := sink.Records(); len(recs) != 0 {
		t.Errorf("ordinary redacted read must emit no audit record, got %d: %+v", len(recs), recs)
	}
}

// A reveal is recorded because the PERMISSION was exercised, not because it
// happened to uncover something. An entity with nothing redacted still took the
// override path, and an auditor asking "when was this permission used" needs it.
func TestHistoryReveal_AuditedEvenWhenNothingWasHidden(t *testing.T) {
	sink := &audit.Memory{}
	app := buildPolicyApp(t, historyRedactionACL, sink)
	seedEntity(app, &entity.Entity{
		ID:         "TKT-002",
		Type:       "ticket",
		Properties: map[string]any{"title": "t", "status": "open"},
	})
	app.versions = historyStore{
		versions: map[string][]store.VersionSnapshot{
			"TKT-002": {snapshot("ticket", "body",
				map[string]any{"title": "t", "status": "open"})},
		},
	}
	gate := permGate{perms: map[string]bool{
		acl.PermHistoryRead:         true,
		acl.PermHistoryReadRedacted: true,
	}}

	decodeHistoryEntity(t, getHistoryVersion(app, "alice", "TKT-002", gate))

	if recs := sink.Records(); len(recs) != 1 {
		t.Errorf("reveal with nothing hidden must still be recorded, got %d records", len(recs))
	}
}

// Under NO ACL, nopReadGate.HoldsPermission returns true for every permission
// (readgate.go:135) — so every history read takes the reveal arm. But with no
// policy configured nothing is redacted, so those reads reveal NOTHING: the
// "reveal" is an artifact of the permit-all gate, not a privileged disclosure.
//
// Recording them would fill the audit log of every unconfigured deployment with
// history-reveal rows that mean nothing, and — worse — would train an operator
// who later configures a policy to ignore exactly the row this ticket exists to
// make visible.
func TestHistoryReveal_NoACL_NotAudited(t *testing.T) {
	sink := &audit.Memory{}
	// NopACL, not buildPolicyApp: this is the genuinely unconfigured
	// deployment, where readGateFromContext hands back nopReadGate.
	app := buildAppWithACLAndAudit(t, acl.NopACL{}, sink)
	seedEntity(app, &entity.Entity{
		ID:         "TKT-001",
		Type:       historyEntityType,
		Properties: map[string]any{"title": "t", "priority": "high", "status": "open"},
	})
	app.versions = historyStore{
		versions: map[string][]store.VersionSnapshot{
			"TKT-001": {snapshot(historyEntityType, "body",
				map[string]any{"title": "t", "priority": "high", "status": "open"})},
		},
	}

	req := httptest.NewRequest(http.MethodGet,
		"/api/v1/_history/"+historyEntityType+"/TKT-001/1", http.NoBody)
	req = req.WithContext(principal.With(req.Context(),
		principal.Principal{User: "alice", Tool: principal.ToolDataEntry}))
	rec := httptest.NewRecorder()
	handleV1History(app, rec, req)

	// Precondition: no ACL means nothing is redacted, so this read genuinely
	// discloses nothing that any policy was withholding.
	hist := decodeHistoryEntity(t, rec)
	if got := hist.Properties["priority"]; got != "high" {
		t.Fatalf("precondition: with no ACL every field should serve, got priority=%v", got)
	}

	if recs := sink.Records(); len(recs) != 0 {
		t.Errorf("no-ACL history read must not be recorded as a reveal (nothing is redacted, "+
			"so nothing is revealed), got %d: %+v", len(recs), recs)
	}
}
