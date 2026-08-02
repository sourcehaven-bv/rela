package dataentry

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// TKT-B1F5Q1 — relation field-level `visible:` redaction end-to-end: a relation
// grant hides selected meta keys on the live relation GET. Relation HISTORY
// redaction is governed by the current live world against the live source (see
// the scenario suite lower in this file); it is deliberately NOT the entity
// history's reconstruct-and-reveal model (IB-review #1).

// relationRedactionACL grants `owner` visibility of the `reason` meta key on a
// `depends_on` relation but NOT `secret` — closed-world so `secret` is hidden.
// The whole grant is unconditional so the live path shows a clean selective
// strip; the historical fail-closed cases use a conditional variant below.
const relationRedactionACL = `
roles:
  owner:
    read:
      - ticket
    relations:
      ticket:
        - relation: depends_on
          visible:
            - field: reason
assignments:
  alice: owner
`

// getRelations drives handleV1EntityRelations for id under the named principal
// and returns the decoded relations map (relType → []rel).
func getRelations(t *testing.T, app *App, user, typeName, id string) map[string][]map[string]any {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/tickets/"+id+"/relations", http.NoBody)
	req = req.WithContext(principal.With(req.Context(),
		principal.Principal{User: user, Tool: principal.ToolDataEntry}))
	rec := httptest.NewRecorder()
	app.handleV1EntityRelations(rec, req, typeName, id)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET relations %s: got %d, want 200; body=%s", id, rec.Code, rec.Body.String())
	}
	var out map[string][]map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&out); err != nil {
		t.Fatalf("decode relations: %v", err)
	}
	return out
}

// Live relation GET: a granted meta key (reason) is visible; a non-granted key
// (secret) is stripped by the closed-world relation `visible:` block.
func TestRelationRedaction_LiveGet_SelectiveStrip(t *testing.T) {
	app := buildPolicyApp(t, relationRedactionACL, nil)
	seedEntity(app, &entity.Entity{ID: "TKT-001", Type: "ticket", Properties: map[string]any{"title": "a"}})
	seedEntity(app, &entity.Entity{ID: "TKT-002", Type: "ticket", Properties: map[string]any{"title": "b"}})
	if _, err := app.store.CreateRelation(context.Background(), "TKT-001", "depends_on", "TKT-002",
		&store.RelationData{Properties: map[string]any{"reason": "blocked", "secret": "s3cr3t"}}); err != nil {
		t.Fatalf("CreateRelation: %v", err)
	}

	rels := getRelations(t, app, "alice", "ticket", "TKT-001")
	group := rels["depends_on"]
	if len(group) != 1 {
		t.Fatalf("expected 1 depends_on edge, got %d (%v)", len(group), rels)
	}
	meta, _ := group[0]["meta"].(map[string]any)
	if _, ok := meta["secret"]; ok {
		t.Errorf("secret meta must be redacted (closed-world), got %v", meta["secret"])
	}
	if got := meta["reason"]; got != "blocked" {
		t.Errorf("reason meta should be visible=blocked, got %v", got)
	}
}

// Live incoming relation GET: redaction resolves the grant against the SOURCE
// entity of the edge (the peer, not the path entity). Viewing TKT-002's incoming
// depends_on edge still strips `secret`, because the grant lives on TKT-001
// (the from side).
func TestRelationRedaction_LiveIncoming_UsesSourceGrant(t *testing.T) {
	app := buildPolicyApp(t, relationRedactionACL, nil)
	seedEntity(app, &entity.Entity{ID: "TKT-001", Type: "ticket", Properties: map[string]any{"title": "a"}})
	seedEntity(app, &entity.Entity{ID: "TKT-002", Type: "ticket", Properties: map[string]any{"title": "b"}})
	if _, err := app.store.CreateRelation(context.Background(), "TKT-001", "depends_on", "TKT-002",
		&store.RelationData{Properties: map[string]any{"reason": "blocked", "secret": "s3cr3t"}}); err != nil {
		t.Fatalf("CreateRelation: %v", err)
	}

	// View from TKT-002 (the TO side): the incoming edge appears under the inverse
	// key. The grant is keyed on TKT-001's type, resolved via relationSourceEntity.
	req := httptest.NewRequest(http.MethodGet, "/api/v1/tickets/TKT-002/relations", http.NoBody)
	req = req.WithContext(principal.With(req.Context(),
		principal.Principal{User: "alice", Tool: principal.ToolDataEntry}))
	rec := httptest.NewRecorder()
	app.handleV1EntityRelations(rec, req, "ticket", "TKT-002")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET relations TKT-002: got %d; body=%s", rec.Code, rec.Body.String())
	}
	var out map[string][]map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	var found bool
	for _, group := range out {
		for _, rel := range group {
			if rel["id"] != "TKT-001" {
				continue
			}
			found = true
			meta, _ := rel["meta"].(map[string]any)
			if _, ok := meta["secret"]; ok {
				t.Errorf("incoming edge: secret must be redacted via source grant, got %v", meta["secret"])
			}
			if got := meta["reason"]; got != "blocked" {
				t.Errorf("incoming edge: reason should be visible, got %v", got)
			}
		}
	}
	if !found {
		t.Fatalf("incoming depends_on edge from TKT-001 not present: %v", out)
	}
}

// A relation type with no `visible:` block is emitted un-redacted (permissive):
// belongs_to carries no grant, so all its meta survives.
func TestRelationRedaction_NoBlock_Permissive(t *testing.T) {
	app := buildPolicyApp(t, relationRedactionACL, nil)
	seedEntity(app, &entity.Entity{ID: "TKT-001", Type: "ticket", Properties: map[string]any{"title": "a"}})
	seedEntity(app, &entity.Entity{ID: "CMP-1", Type: "component", Properties: map[string]any{"name": "c"}})
	if _, err := app.store.CreateRelation(context.Background(), "TKT-001", "belongs_to", "CMP-1",
		&store.RelationData{Properties: map[string]any{"note": "keep me"}}); err != nil {
		t.Fatalf("CreateRelation: %v", err)
	}
	rels := getRelations(t, app, "alice", "ticket", "TKT-001")
	group := rels["belongs_to"]
	if len(group) != 1 {
		t.Fatalf("expected 1 belongs_to edge, got %d", len(group))
	}
	meta, _ := group[0]["meta"].(map[string]any)
	if got := meta["note"]; got != "keep me" {
		t.Errorf("belongs_to has no visible: block → note must survive, got %v", got)
	}
}

func decodeRelHistoryMeta(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	if rec.Code != http.StatusOK {
		t.Fatalf("relation history version: got %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	var resp struct {
		Relation struct {
			Meta map[string]any `json:"meta"`
		} `json:"relation"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode relation history: %v", err)
	}
	return resp.Relation.Meta
}

// ---------------------------------------------------------------------------
// Relation history redaction is governed by the CURRENT LIVE world, evaluated
// against the LIVE source entity (TKT-B1F5Q1, IB-review #1). Scenarios 1-7 pin
// the agreed behavior:
//   1. live source, reader entitled           → field visible (per-field redact)
//   2. live source, reader loses entitlement   → field hidden
//   3. live source, reader gains entitlement    → field visible
//   4. relation deleted (source still live)     → served against live source
//   5. source entity deleted                    → NO meta to anyone (no reveal)
//   6. `visible:` grant removed from policy      → field hidden (policy is live)
//   7. property renamed since capture            → over-redacts, never leaks
//
// The running example: relation SALARY (alice, depends_on, acme) with meta
// {reason, secret}. Role `hr` gets `visible: reason` on depends_on under type
// `ticket`; `reason` is per-field visible to HR, `secret` never granted.

// hrVisibleACL — an HR-only reader-side grant. current_user.id gates it so a
// test can flip a reader's entitlement without touching the graph.
const hrVisibleACL = `
roles:
  hr:
    read:
      - ticket
    relations:
      ticket:
        - relation: depends_on
          visible:
            - field: reason
              when: "current_user.id == 'hr_reader'"
assignments:
  hr_reader: hr
  plain_reader: hr
`

// seedLiveRelHistoryApp builds a policy app with a LIVE depends_on relation
// (alice→acme) carrying {reason, secret}, plus a matching history snapshot. Both
// endpoints are live entities of type `ticket`.
func seedLiveRelHistoryApp(t *testing.T, aclYAML string) *App {
	t.Helper()
	app := buildPolicyApp(t, aclYAML, nil)
	seedEntity(app, &entity.Entity{ID: "alice", Type: "ticket", Properties: map[string]any{"title": "alice"}})
	seedEntity(app, &entity.Entity{ID: "acme", Type: "ticket", Properties: map[string]any{"title": "acme"}})
	if _, err := app.store.CreateRelation(context.Background(), "alice", "depends_on", "acme",
		&store.RelationData{Properties: map[string]any{"reason": "layoff", "secret": "flight risk"}}); err != nil {
		t.Fatalf("CreateRelation depends_on: %v", err)
	}
	app.versions = relHistoryStore{
		versions: map[string][]store.RelationVersionSnapshot{
			relKey("alice", "depends_on", "acme"): {{
				RelationVersionMeta: store.RelationVersionMeta{
					Version: 1, Op: store.VersionOpCreate, From: "alice", Type: "depends_on", To: "acme",
				},
				Content:    "body",
				Properties: map[string]any{"reason": "layoff", "secret": "flight risk"},
			}},
		},
	}
	return app
}

// getRelHistoryMetaAs drives the history version GET for alice→depends_on→acme as
// `user` (read gate permits reads; history:read held) and returns the meta map.
func getRelHistoryMetaAs(t *testing.T, app *App, user string) map[string]any {
	t.Helper()
	perms := map[string]bool{acl.PermHistoryRead: true}
	req := httptest.NewRequest(http.MethodGet,
		"/api/v1/_relation_history/ticket/alice/depends_on/acme/1", http.NoBody)
	ctx := principal.With(req.Context(), principal.Principal{User: user, Tool: principal.ToolDataEntry})
	ctx = withReadGate(ctx, permGate{perms: perms})
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()
	handleV1RelationHistory(app, rec, req)
	return decodeRelHistoryMeta(t, rec)
}

// Scenario 1 — live source, reader is HR → `reason` visible, `secret` hidden.
func TestRelHistory_S1_LiveSource_EntitledReader_PerFieldRedact(t *testing.T) {
	app := seedLiveRelHistoryApp(t, hrVisibleACL)
	meta := getRelHistoryMetaAs(t, app, "hr_reader")
	if got := meta["reason"]; got != "layoff" {
		t.Errorf("HR reader should see reason=layoff, got %v", got)
	}
	if _, ok := meta["secret"]; ok {
		t.Errorf("secret is never granted → must be hidden, got %v", meta["secret"])
	}
}

// Scenario 2 — live source, reader is NOT HR → `reason` hidden (reader-side is live).
func TestRelHistory_S2_LiveSource_UnentitledReader_Hidden(t *testing.T) {
	app := seedLiveRelHistoryApp(t, hrVisibleACL)
	meta := getRelHistoryMetaAs(t, app, "plain_reader")
	if _, ok := meta["reason"]; ok {
		t.Errorf("non-HR reader must not see reason, got %v", meta["reason"])
	}
}

// Scenario 3 — the same version, two readers: whoever currently holds the grant
// sees it. This is the "gain/lose a role takes effect on history immediately"
// property — reader-side access is always evaluated live, both directions.
func TestRelHistory_S3_LiveSource_ReaderEntitlementIsLive(t *testing.T) {
	app := seedLiveRelHistoryApp(t, hrVisibleACL)
	if got := getRelHistoryMetaAs(t, app, "hr_reader")["reason"]; got != "layoff" {
		t.Errorf("entitled reader sees reason, got %v", got)
	}
	if _, ok := getRelHistoryMetaAs(t, app, "plain_reader")["reason"]; ok {
		t.Errorf("unentitled reader does not, got a value")
	}
}

// Scenario 4 — the relation is deleted but its SOURCE entity is still live. The
// source governs access, so redaction still resolves per-field against it (HR
// sees reason). Uses an unconditional grant so the only variable is source
// liveness. History carries the (now-deleted) relation; alice still exists.
func TestRelHistory_S4_RelationDeleted_SourceLive_ResolvesAgainstSource(t *testing.T) {
	const uncondACL = `
roles:
  hr:
    read:
      - ticket
    relations:
      ticket:
        - relation: depends_on
          visible:
            - field: reason
assignments:
  hr_reader: hr
`
	app := buildPolicyApp(t, uncondACL, nil)
	seedEntity(app, &entity.Entity{ID: "alice", Type: "ticket", Properties: map[string]any{"title": "alice"}})
	seedEntity(app, &entity.Entity{ID: "acme", Type: "ticket", Properties: map[string]any{"title": "acme"}})
	// No live depends_on edge — only history (the relation was deleted). alice lives.
	app.versions = relHistoryStore{
		versions: map[string][]store.RelationVersionSnapshot{
			relKey("alice", "depends_on", "acme"): {{
				RelationVersionMeta: store.RelationVersionMeta{
					Version: 1, Op: store.VersionOpDelete, From: "alice", Type: "depends_on", To: "acme",
				},
				Content:    "body",
				Properties: map[string]any{"reason": "layoff", "secret": "flight risk"},
			}},
		},
	}
	meta := getRelHistoryMetaAs(t, app, "hr_reader")
	if got := meta["reason"]; got != "layoff" {
		t.Errorf("deleted relation with LIVE source: HR should still see reason (source governs), got %v", got)
	}
	if _, ok := meta["secret"]; ok {
		t.Errorf("secret still hidden, got %v", meta["secret"])
	}
}

// Scenario 5 — the SOURCE entity is deleted. No live relation, no resolvable
// type → NO meta to anyone, including history:read-redacted (no reveal for a
// deleted relation — gone is gone). This is the CISO IB-review #1 fix, in two
// flavors: a non-matching fromType, and a fromType spoofed to the attacker's own
// grant. Both must serve nothing.
func TestRelHistory_S5_SourceDeleted_NoMetaEvenWithReveal(t *testing.T) {
	// alice holds an UNCONDITIONAL grant on depends_on under `ticket`. If the
	// handler trusted the URL fromType, spoofing fromType=ticket for a deleted
	// source would leak reason. It must not.
	const ownGrantACL = `
roles:
  hr:
    relations:
      ticket:
        - relation: depends_on
          visible:
            - field: reason
assignments:
  hr_reader: hr
`
	app := buildPolicyApp(t, ownGrantACL, nil)
	app.versions = relHistoryStore{
		versions: map[string][]store.RelationVersionSnapshot{
			relKey("GONE-A", "depends_on", "GONE-B"): {{
				RelationVersionMeta: store.RelationVersionMeta{
					Version: 1, Op: store.VersionOpDelete, From: "GONE-A", Type: "depends_on", To: "GONE-B",
				},
				Content:    "body",
				Properties: map[string]any{"reason": "leaked?", "secret": "s3cr3t"},
			}},
		},
	}

	cases := []struct {
		name     string
		fromType string
		perms    []string
	}{
		{"non-matching fromType", "component", nil},
		{"spoofed to own grant type", "ticket", nil},
		{"spoofed + history:read-redacted (no reveal for deleted)", "ticket", []string{acl.PermHistoryReadRedacted}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			perms := map[string]bool{acl.PermHistoryRead: true}
			for _, p := range tc.perms {
				perms[p] = true
			}
			req := httptest.NewRequest(http.MethodGet,
				"/api/v1/_relation_history/"+tc.fromType+"/GONE-A/depends_on/GONE-B/1", http.NoBody)
			ctx := principal.With(req.Context(), principal.Principal{User: "hr_reader", Tool: principal.ToolDataEntry})
			ctx = withReadGate(ctx, permGate{perms: perms})
			req = req.WithContext(ctx)
			rec := httptest.NewRecorder()
			handleV1RelationHistory(app, rec, req)
			meta := decodeRelHistoryMeta(t, rec)
			if len(meta) != 0 {
				t.Errorf("deleted source must serve NO meta, got %v", meta)
			}
		})
	}
}

// Scenario 6 — policy is read LIVE. A version captured while a `visible:` grant
// existed is hidden once that grant is removed from acl.yaml. (Here the policy
// simply never grants reason, standing in for "grant removed" — the resolver has
// no memory of a past policy.)
func TestRelHistory_S6_PolicyIsLive_RemovedGrantHides(t *testing.T) {
	const noGrantACL = `
roles:
  hr:
    read:
      - ticket
    relations:
      ticket:
        - relation: depends_on
          visible:
            - field: something_else
assignments:
  hr_reader: hr
`
	app := seedLiveRelHistoryApp(t, noGrantACL)
	meta := getRelHistoryMetaAs(t, app, "hr_reader")
	if _, ok := meta["reason"]; ok {
		t.Errorf("policy no longer grants reason → must be hidden, got %v", meta["reason"])
	}
}

// Scenario 7 — a `visible:` grant referencing a property the frozen record does
// not carry over-redacts, never leaks. Grant is on `reason` but conditioned on a
// property (`title`) that the live source has; the point is that a grant naming a
// field absent from the frozen meta simply grants nothing for it, and unrelated
// present keys stay closed-world hidden.
func TestRelHistory_S7_DriftedProperty_OverRedactsNeverLeaks(t *testing.T) {
	// Grant visible on `note` (a key NOT present in the frozen meta). The frozen
	// meta carries reason+secret; neither is granted → both hidden. A drifted grant
	// never accidentally reveals a present-but-ungranted key.
	const driftACL = `
roles:
  hr:
    read:
      - ticket
    relations:
      ticket:
        - relation: depends_on
          visible:
            - field: note
assignments:
  hr_reader: hr
`
	app := seedLiveRelHistoryApp(t, driftACL)
	meta := getRelHistoryMetaAs(t, app, "hr_reader")
	if _, ok := meta["reason"]; ok {
		t.Errorf("grant names an absent field → present ungranted keys stay hidden; reason leaked: %v", meta["reason"])
	}
	if _, ok := meta["secret"]; ok {
		t.Errorf("secret leaked: %v", meta["secret"])
	}
}

// Restore reads RAW frozen meta, never the redacted view (CLAUDE.md "never redact
// a read that feeds a write"). A reader who cannot see `secret` on the display
// path restores a version and the restored LIVE relation must still carry secret
// — a redacted read-modify-write would erase it.
func TestRelHistoryRestore_ReadsRawMeta_PreservesHidden(t *testing.T) {
	app := seedLiveRelHistoryApp(t, hrVisibleACL)
	// Display path for plain_reader hides both reason (not HR) and secret.
	displayed := getRelHistoryMetaAs(t, app, "plain_reader")
	if _, ok := displayed["secret"]; ok {
		t.Fatalf("precondition: plain reader should not see secret, got %v", displayed["secret"])
	}

	req := httptest.NewRequest(http.MethodPost,
		"/api/v1/_relation_history/ticket/alice/depends_on/acme/1/restore", http.NoBody)
	ctx := principal.With(req.Context(), principal.Principal{User: "plain_reader", Tool: principal.ToolDataEntry})
	ctx = withReadGate(ctx, permGate{perms: map[string]bool{acl.PermHistoryRead: true}})
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()
	handleV1RelationHistory(app, rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("restore: got %d, want 200; body=%s", rec.Code, rec.Body.String())
	}

	live, err := app.store.GetRelation(context.Background(), "alice", "depends_on", "acme")
	if err != nil {
		t.Fatalf("GetRelation after restore: %v", err)
	}
	if got := live.Properties["secret"]; got != "flight risk" {
		t.Errorf("restore must preserve hidden meta (raw read): secret=%v, want 'flight risk'", got)
	}
}

// Incoming-edge source fail-closed (live path, unchanged by IB-review #1): when an
// incoming edge's source cannot be fetched, visibleRelationMetaIncoming drops the
// whole meta rather than fall back to the wrong-type path entity.
func TestVisibleRelationMetaIncoming_SourceGone_FailsClosed(t *testing.T) {
	app := buildPolicyApp(t, relationRedactionACL, nil)
	svc := app.affordances
	svc.getEntity = func(context.Context, string) (*entity.Entity, bool) { return nil, false }

	meta := map[string]any{"reason": "blocked", "secret": "s3cr3t"}
	got := svc.visibleRelationMetaIncoming(context.Background(), "TKT-001", "depends_on", meta)
	if len(got) != 0 {
		t.Errorf("source gone must fail closed (empty meta), got %v", got)
	}
	if len(meta) != 2 {
		t.Errorf("input meta must be untouched, got %v", meta)
	}
}
