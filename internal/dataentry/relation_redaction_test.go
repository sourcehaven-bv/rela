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
// grant hides selected meta keys on the live relation GET, and relation history
// inherits the same deny-by-default fail-closed rule + history:read-redacted
// reveal from TKT-73C6B2.

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

// getRelHistoryVersion drives serveRelationHistoryVersion via handleV1RelationHistory.
func getRelHistoryVersion(app *App, user string, gate readGate) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet,
		"/api/v1/_relation_history/ticket/TKT-001/depends_on/TKT-002/1", http.NoBody)
	ctx := principal.With(req.Context(),
		principal.Principal{User: user, Tool: principal.ToolDataEntry})
	ctx = withReadGate(ctx, gate)
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()
	handleV1RelationHistory(app, rec, req)
	return rec
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

// relationHistoryConditionalACL gates `reason` on a subject-world lookup
// (has_relation) so history fails closed while the live edge still exists.
const relationHistoryConditionalACL = `
roles:
  owner:
    read:
      - ticket
    relations:
      ticket:
        - relation: depends_on
          visible:
            - field: reason
              when: "has_relation(entity, 'blocks')"
assignments:
  alice: owner
`

// seedConditionalRelHistoryApp builds a policy app whose live TKT-001 has a
// `blocks` edge (so the live grant passes) plus a canned relation-history
// snapshot carrying `reason`.
func seedConditionalRelHistoryApp(t *testing.T) *App {
	t.Helper()
	app := buildPolicyApp(t, relationHistoryConditionalACL, nil)
	seedEntity(app, &entity.Entity{ID: "TKT-001", Type: "ticket", Properties: map[string]any{"title": "a"}})
	seedEntity(app, &entity.Entity{ID: "TKT-002", Type: "ticket", Properties: map[string]any{"title": "b"}})
	seedEntity(app, &entity.Entity{ID: "TKT-009", Type: "ticket", Properties: map[string]any{"title": "c"}})
	// Live blocks edge makes the grant pass on a LIVE read.
	if _, err := app.store.CreateRelation(context.Background(), "TKT-001", "blocks", "TKT-009", nil); err != nil {
		t.Fatalf("CreateRelation blocks: %v", err)
	}
	// Live depends_on edge carrying reason, so the live sanity read has an edge to
	// evaluate the grant against (the history snapshot below is the SAME triple).
	if _, err := app.store.CreateRelation(context.Background(), "TKT-001", "depends_on", "TKT-002",
		&store.RelationData{Properties: map[string]any{"reason": "blocked"}}); err != nil {
		t.Fatalf("CreateRelation depends_on: %v", err)
	}
	app.versions = relHistoryStore{
		versions: map[string][]store.RelationVersionSnapshot{
			relKey("TKT-001", "depends_on", "TKT-002"): {{
				RelationVersionMeta: store.RelationVersionMeta{
					Version: 1, Op: store.VersionOpCreate, From: "TKT-001", Type: "depends_on", To: "TKT-002",
				},
				Content:    "body",
				Properties: map[string]any{"reason": "blocked"},
			}},
		},
	}
	return app
}

// Relation history fails closed: a subject-conditional `reason` grant that is
// affirmable on the LIVE edge is HIDDEN in the historical snapshot (the marker
// neuters the subject-world lookup). Mirrors the entity SubjectConditional test.
func TestRelationHistoryRedaction_SubjectConditional_FailsClosed(t *testing.T) {
	app := seedConditionalRelHistoryApp(t)

	// Sanity: LIVE relation GET shows reason (blocks edge present).
	rels := getRelations(t, app, "alice", "ticket", "TKT-001")
	var liveReasonShown bool
	for _, rel := range rels["depends_on"] {
		if meta, _ := rel["meta"].(map[string]any); meta["reason"] == "blocked" {
			liveReasonShown = true
		}
	}
	if !liveReasonShown {
		t.Fatalf("live: reason should be visible (blocks edge present); rels=%v", rels)
	}

	// Historical read (no reveal permission): reason must be stripped.
	gate := permGate{perms: map[string]bool{acl.PermHistoryRead: true}}
	meta := decodeRelHistoryMeta(t, getRelHistoryVersion(app, "alice", gate))
	if _, ok := meta["reason"]; ok {
		t.Errorf("historical relation read must fail closed: reason should be stripped, got %v", meta["reason"])
	}
}

// A holder of history:read-redacted sees the frozen relation meta the ordinary
// reader has redacted — OVERRIDE reveal, exactly as the entity path.
func TestRelationHistoryRedaction_RevealPermission_ShowsFrozenMeta(t *testing.T) {
	app := seedConditionalRelHistoryApp(t)

	gate := permGate{perms: map[string]bool{
		acl.PermHistoryRead:         true,
		acl.PermHistoryReadRedacted: true,
	}}
	meta := decodeRelHistoryMeta(t, getRelHistoryVersion(app, "alice", gate))
	if got, ok := meta["reason"]; !ok || got != "blocked" {
		t.Errorf("reveal: reason should be present=blocked, got ok=%v v=%v", ok, got)
	}
}

// CLAUDE.md rule ("Never redact a read that feeds a write"): relation restore must
// read the RAW frozen meta, not the redacted view. A non-history:read-redacted
// principal restores a version whose meta is currently redacted on the display
// path; the restored LIVE edge must still carry the hidden key — a redacted
// read-modify-write would erase it (RR-B1F5-S1). Pins the two-handles invariant so
// a future "tidy the GetRelationVersion calls into one helper" refactor can't
// silently reintroduce the data-destruction bug.
func TestRelationHistoryRestore_ReadsRawMeta_PreservesHidden(t *testing.T) {
	app := seedConditionalRelHistoryApp(t)
	// The live depends_on edge currently carries reason (seeded). Sanity: on the
	// history DISPLAY path (no reveal), reason is redacted — so if restore used the
	// display read it would drop reason on save.
	gate := permGate{perms: map[string]bool{acl.PermHistoryRead: true}}
	displayed := decodeRelHistoryMeta(t, getRelHistoryVersion(app, "alice", gate))
	if _, ok := displayed["reason"]; ok {
		t.Fatalf("precondition: display path should redact reason, got %v", displayed["reason"])
	}

	// Restore version 1 as the SAME non-reveal principal.
	req := httptest.NewRequest(http.MethodPost,
		"/api/v1/_relation_history/ticket/TKT-001/depends_on/TKT-002/1/restore", http.NoBody)
	ctx := principal.With(req.Context(), principal.Principal{User: "alice", Tool: principal.ToolDataEntry})
	ctx = withReadGate(ctx, gate)
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()
	handleV1RelationHistory(app, rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("restore: got %d, want 200; body=%s", rec.Code, rec.Body.String())
	}

	// The restored LIVE relation must still carry reason — restore read it raw.
	live, err := app.store.GetRelation(context.Background(), "TKT-001", "depends_on", "TKT-002")
	if err != nil {
		t.Fatalf("GetRelation after restore: %v", err)
	}
	if got := live.Properties["reason"]; got != "blocked" {
		t.Errorf("restore must preserve hidden meta (raw read): reason=%v, want blocked", got)
	}
}

// N1 (incoming-source fail-closed): when an incoming edge's source entity cannot
// be fetched (deleted mid-request), visibleRelationMetaIncoming must drop the
// whole meta rather than fall back to the wrong-type path entity and emit it
// un-redacted. Exercises the helper directly with a getEntity that reports the
// peer gone, against a policy resolver that CAN redact (so the fail-closed branch,
// not the Nop passthrough, is taken).
func TestVisibleRelationMetaIncoming_SourceGone_FailsClosed(t *testing.T) {
	app := buildPolicyApp(t, relationRedactionACL, nil)
	// Override getEntity so the source (TKT-001) looks deleted.
	svc := app.affordances
	svc.getEntity = func(context.Context, string) (*entity.Entity, bool) { return nil, false }

	meta := map[string]any{"reason": "blocked", "secret": "s3cr3t"}
	got := svc.visibleRelationMetaIncoming(context.Background(), "TKT-001", "depends_on", meta)
	if len(got) != 0 {
		t.Errorf("source gone must fail closed (empty meta), got %v", got)
	}
	// And it must not have mutated the input.
	if len(meta) != 2 {
		t.Errorf("input meta must be untouched, got %v", meta)
	}
}

// S2 (deleted-endpoint fail-closed): a deleted relation's history must fail closed
// on meta even though the source entity is gone and fromType is a caller-supplied
// URL segment that is NOT validated against a live row. The type-level
// closed-world keys on the RELATION type (not the synthesized from.Type), so a
// spoofed/non-matching fromType still hides the meta. Non-holder of
// history:read-redacted → reason stripped.
func TestRelationHistoryRedaction_DeletedEndpoint_FailsClosedDespiteFromType(t *testing.T) {
	// No live endpoints; canned history for a gone depends_on relation carrying reason.
	app := buildPolicyApp(t, relationHistoryConditionalACL, nil)
	app.versions = relHistoryStore{
		versions: map[string][]store.RelationVersionSnapshot{
			relKey("GONE-A", "depends_on", "GONE-B"): {{
				RelationVersionMeta: store.RelationVersionMeta{
					Version: 1, Op: store.VersionOpDelete, From: "GONE-A", Type: "depends_on", To: "GONE-B",
				},
				Content:    "body",
				Properties: map[string]any{"reason": "blocked"},
			}},
		},
	}

	// fromType in the URL ("bogus_type") does not match any visible:-declaring type;
	// deleted endpoints → gated on history:read only (no per-type verdict).
	req := httptest.NewRequest(http.MethodGet,
		"/api/v1/_relation_history/bogus_type/GONE-A/depends_on/GONE-B/1", http.NoBody)
	ctx := principal.With(req.Context(), principal.Principal{User: "alice", Tool: principal.ToolDataEntry})
	// Holds history:read (authorizes deleted-relation history) but NOT
	// history:read-redacted (so the fail-closed strip must run).
	ctx = withReadGate(ctx, permGate{perms: map[string]bool{acl.PermHistoryRead: true}})
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()
	handleV1RelationHistory(app, rec, req)
	meta := decodeRelHistoryMeta(t, rec)
	if _, ok := meta["reason"]; ok {
		t.Errorf("deleted-endpoint history must fail closed on reason regardless of fromType, got %v", meta["reason"])
	}
}
