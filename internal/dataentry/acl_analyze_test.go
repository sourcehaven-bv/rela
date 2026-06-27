package dataentry

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/entity"
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

func countWarnings(issues []APIIssue) int {
	n := 0
	for _, i := range issues {
		if i.Severity == "warning" {
			n++
		}
	}
	return n
}
