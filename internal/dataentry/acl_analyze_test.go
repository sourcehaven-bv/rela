package dataentry

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// TestACLAnalyze_FiltersHiddenIssues pins TKT-QU7REX: _analyze walks the whole
// graph, so every issue's entityId/entityType/title would leak to a principal
// who cannot read that entity. A ticket-only viewer running _analyze must see
// issues for tickets but NOT for entities of denied types — and the aggregate
// counts must reflect only the visible subset.
func TestACLAnalyze_FiltersHiddenIssues(t *testing.T) {
	app := newTestAppV1(t)
	// Two orphan entities (no relations → each trips the orphans warning).
	// alice (viewer) can read tickets but not features.
	seedEntity(app, &entity.Entity{
		ID: "TKT-001", Type: "ticket",
		Properties: map[string]any{"title": "visible ticket"},
	})
	seedEntity(app, &entity.Entity{
		ID: "FEAT-SECRET", Type: "feature",
		Properties: map[string]any{"title": "hidden feature title"},
	})

	d := mustNewACL(t, &acl.Policy{
		Roles:       map[string]acl.RoleDef{"viewer": {Read: []string{"ticket"}}},
		Assignments: map[string]string{"alice": "viewer"},
	}, app.store)
	app.acl = d

	req := httptest.NewRequest(http.MethodGet, "/api/v1/_analyze", http.NoBody)
	req = req.WithContext(gateCtxFor(aliceCtx(), t, d))
	rec := httptest.NewRecorder()
	app.handleV1Analyze(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("_analyze: got %d, want 200; body=%s", rec.Code, rec.Body)
	}
	body := rec.Body.String()

	// The hidden feature must not appear anywhere — not its id, not its title.
	if strings.Contains(body, "FEAT-SECRET") || strings.Contains(body, "hidden feature title") {
		t.Errorf("LEAK: _analyze exposed a denied entity to a ticket-only viewer: %s", body)
	}

	// The visible ticket's issue should still be present.
	var result APIAnalysisResult
	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		t.Fatalf("decode _analyze response: %v", err)
	}
	sawTicket := false
	for _, iss := range result.Issues {
		if iss.EntityID == "FEAT-SECRET" {
			t.Errorf("LEAK: denied feature issue present in decoded issues: %+v", iss)
		}
		if iss.EntityID == "TKT-001" {
			sawTicket = true
		}
	}
	if !sawTicket {
		t.Errorf("expected the visible ticket's orphan issue to remain, got issues: %+v", result.Issues)
	}

	// Counts must reflect only visible issues: with the feature filtered out,
	// no issue may reference it, and the warning count equals len(visible
	// warnings) — at minimum the ticket's, never the feature's.
	if result.Warnings != countWarnings(result.Issues) {
		t.Errorf("Warnings count %d disagrees with visible issue list (%d) — aggregate leaks hidden issues",
			result.Warnings, countWarnings(result.Issues))
	}
}

// TestACLAnalyze_RedactsHiddenPrimaryTitle pins the field-level half of the
// analyze surface (BUG-R9EHKV): an entity the principal CAN read but whose
// display (primary) property is hidden by `visible:` must have its analyze
// issue title fall back to the id, not leak the hidden value. TKT-QU7REX already
// drops issues for unreadable rows; this covers the readable-but-redacted case.
func TestACLAnalyze_RedactsHiddenPrimaryTitle(t *testing.T) {
	app := newTestAppV1(t)
	// title is the ticket's display property; hide it for everyone.
	app.fieldResolver = fakeResolver{fv: FieldVerdicts{
		Visible: map[string]bool{"title": false},
	}}
	// Orphan ticket → trips the orphans warning, producing an issue with a title.
	seedEntity(app, &entity.Entity{
		ID: "TKT-001", Type: "ticket",
		Properties: map[string]any{"title": "SECRET-ANALYZE-TITLE"},
	})

	d := mustNewACL(t, &acl.Policy{
		Roles:       map[string]acl.RoleDef{"viewer": {Read: []string{"ticket"}}},
		Assignments: map[string]string{"alice": "viewer"},
	}, app.store)
	app.acl = d

	req := httptest.NewRequest(http.MethodGet, "/api/v1/_analyze", http.NoBody)
	req = req.WithContext(gateCtxFor(aliceCtx(), t, d))
	rec := httptest.NewRecorder()
	app.handleV1Analyze(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("_analyze: got %d, want 200; body=%s", rec.Code, rec.Body)
	}
	if body := rec.Body.String(); strings.Contains(body, "SECRET-ANALYZE-TITLE") {
		t.Errorf("LEAK: hidden primary property leaked as an analyze issue title: %s", body)
	}
}

// TestACLAnalyze_RedactsHiddenTemplatedTitle pins the TEMPLATED display_property
// case (review finding on the R9EHKV analyze fix): a type whose title is
// `{voornaam} {achternaam}` has no single "primary" property, so a check built
// on GetPrimaryProperty misses it. When a template PLACEHOLDER is hidden, the
// whole analyze title must fall back to the id — a partial render ("Jeroen ")
// would leak the readable half and confirm a hidden half.
func TestACLAnalyze_RedactsHiddenTemplatedTitle(t *testing.T) {
	meta := &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"persoon": {
				Label:           "Persoon",
				IDPrefix:        "PERS-",
				DisplayProperty: "{voornaam} {achternaam}",
				Properties: map[string]metamodel.PropertyDef{
					"voornaam":   {Type: "string", Required: true},
					"achternaam": {Type: "string", Required: true},
				},
				PropertyOrder: []string{"voornaam", "achternaam"},
			},
		},
	}
	app := newAppFromParts(&Config{}, meta, newFixture())
	app.broker = newEventBroker()
	// Hide the achternaam placeholder for everyone.
	app.fieldResolver = fakeResolver{fv: FieldVerdicts{
		Visible: map[string]bool{"achternaam": false},
	}}
	// Orphan persoon → orphans warning with a templated title.
	seedEntity(app, &entity.Entity{
		ID: "PERS-001", Type: "persoon",
		Properties: map[string]any{"voornaam": "Jeroen", "achternaam": "SECRET-SURNAME"},
	})

	d := mustNewACL(t, &acl.Policy{
		Roles:       map[string]acl.RoleDef{"viewer": {Read: []string{"persoon"}}},
		Assignments: map[string]string{"alice": "viewer"},
	}, app.store)
	app.acl = d

	req := httptest.NewRequest(http.MethodGet, "/api/v1/_analyze", http.NoBody)
	req = req.WithContext(gateCtxFor(aliceCtx(), t, d))
	rec := httptest.NewRecorder()
	app.handleV1Analyze(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("_analyze: got %d, want 200; body=%s", rec.Code, rec.Body)
	}
	var result APIAnalysisResult
	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		t.Fatalf("decode _analyze response: %v", err)
	}
	var sawPersoon bool
	for _, iss := range result.Issues {
		if iss.EntityID != "PERS-001" {
			continue
		}
		sawPersoon = true
		// The hidden surname must not leak — AND neither may the PARTIAL title
		// "Jeroen" (readable half). When any template placeholder is hidden the
		// whole title must fall back to the id (tschmits review, TKT-3FL2S6):
		// a partial render leaks the readable half and confirms a hidden half.
		if iss.Title != "PERS-001" {
			t.Errorf("PARTIAL-TITLE LEAK: templated title with a hidden placeholder must fall back to id, got %q", iss.Title)
		}
	}
	if !sawPersoon {
		t.Fatalf("expected an issue for PERS-001; got %+v", result.Issues)
	}
	if body := rec.Body.String(); strings.Contains(body, "SECRET-SURNAME") || strings.Contains(body, "Jeroen") {
		t.Errorf("LEAK: hidden/partial template value in analyze response: %s", body)
	}
}

// TestACLAnalyze_GatedReadClosesMessageLeak pins the leak the wire-boundary
// sentinel caught and the whole gated-analyze arc (TKT-3FL2S6) exists to close:
// the Properties check reports "Invalid value \"X\" (allowed: …)" where X is the
// entity's raw property value. If that property is hidden from the requester, X
// is a value they cannot see — leaked through the issue MESSAGE (not the title).
// Gated reads close it by construction: the entity is redacted before
// ValidateEntity sees it, so there is no bad value to quote.
func TestACLAnalyze_GatedReadClosesMessageLeak(t *testing.T) {
	meta := &metamodel.Metamodel{
		Types: map[string]metamodel.CustomType{
			"status_type": {Values: []string{"open", "closed"}},
		},
		Entities: map[string]metamodel.EntityDef{
			"ticket": {
				Label:    "Ticket",
				IDPrefix: "TKT-",
				Properties: map[string]metamodel.PropertyDef{
					"title":  {Type: "string", Required: true},
					"status": {Type: "status_type"},
				},
				PropertyOrder: []string{"title", "status"},
			},
		},
	}
	app := newAppFromParts(&Config{}, meta, newFixture())
	app.broker = newEventBroker()
	// Hide `status` — the property whose invalid value the Properties check would
	// otherwise quote in its message.
	app.fieldResolver = fakeResolver{fv: FieldVerdicts{
		Visible: map[string]bool{"status": false},
	}}
	// status holds an INVALID enum value → Properties check fires "Invalid value
	// \"SECRET-STATUS\" (allowed: [open closed])".
	seedEntity(app, &entity.Entity{
		ID: "TKT-001", Type: "ticket",
		Properties: map[string]any{"title": "visible title", "status": "SECRET-STATUS"},
	})

	d := mustNewACL(t, &acl.Policy{
		Roles:       map[string]acl.RoleDef{"viewer": {Read: []string{"ticket"}}},
		Assignments: map[string]string{"alice": "viewer"},
	}, app.store)
	app.acl = d

	req := httptest.NewRequest(http.MethodGet, "/api/v1/_analyze", http.NoBody)
	req = req.WithContext(gateCtxFor(aliceCtx(), t, d))
	rec := httptest.NewRecorder()
	app.handleV1Analyze(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("_analyze: got %d, want 200; body=%s", rec.Code, rec.Body)
	}
	if body := rec.Body.String(); strings.Contains(body, "SECRET-STATUS") {
		t.Errorf("LEAK: hidden property value leaked through an analyze issue MESSAGE: %s", body)
	}
}

func countWarnings(issues []APIIssue) int {
	n := 0
	for _, i := range issues {
		if i.Severity == "warning" {
			n++
		}
	}
	return n
}
