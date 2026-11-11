package dataentry

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	"github.com/Sourcehaven-BV/rela/internal/entity"
)

// stubFlowEdits withholds status on every ticket, as a flow's
// `outside_edits:` would while a run holds it. It counts its run lookups: one
// per PrimeRuns, and one per Withheld on a ctx no PrimeRuns covered.
type stubFlowEdits struct {
	lookups atomic.Int64
}

type stubPrimedKey struct{}

func (*stubFlowEdits) Governs(entityType string) bool { return entityType == "ticket" }

func (*stubFlowEdits) Permissions(string) []string { return nil }

func (s *stubFlowEdits) PrimeRuns(ctx context.Context, _ []string) (context.Context, error) {
	s.lookups.Add(1)
	return context.WithValue(ctx, stubPrimedKey{}, true), nil
}

func (s *stubFlowEdits) Withheld(ctx context.Context, _ *entity.Entity, _ func(string) bool) (map[string]string, error) {
	if ctx.Value(stubPrimedKey{}) == nil {
		s.lookups.Add(1)
	}
	return map[string]string{"status": "outside_edits:flow=f state=s"}, nil
}

// flowEditsPolicy lets alice update tickets and declares no field rules, so
// the ceiling is the only field restriction.
const flowEditsPolicy = `
roles:
  editor:
    read: [ticket]
    update: [ticket]
assignments:
  alice: editor
`

// flowEditsApp wires the production resolver with a stub ceiling.
func flowEditsApp(t *testing.T) (*App, *acl.Declarative, *stubFlowEdits) {
	t.Helper()
	app := newTestAppV1(t)
	policy, err := acl.LoadPolicyBytes([]byte(flowEditsPolicy))
	if err != nil {
		t.Fatalf("LoadPolicyBytes: %v", err)
	}
	d := mustNewACL(t, policy, app.store)
	setTestACL(t, app, d)
	ceiling := &stubFlowEdits{}
	res, err := ResolverFromProfile("", app.Meta(), app.store, d, ceiling)
	if err != nil {
		t.Fatalf("ResolverFromProfile: %v", err)
	}
	if _, nop := res.(NopFieldVerdictResolver); nop {
		t.Fatal("a ceiling without field rules chose the Nop resolver")
	}
	setTestResolver(t, app, res)
	return app, d, ceiling
}

// TestFlowEdits_PatchAndFields pins the data-entry surface of a flow's
// outside_edits: the restricted property is read-only in `_fields` and its
// PATCH is refused with the field-affordance rule id, without the
// attribution; an unrestricted property is still written.
func TestFlowEdits_PatchAndFields(t *testing.T) {
	app, _, _ := flowEditsApp(t)
	seedEntity(app, &entity.Entity{ID: "TKT-001", Type: "ticket",
		Properties: map[string]any{"title": "T", "status": "open"}})

	got := getAs(t, app, "alice", "TKT-001")
	if got.FieldAffordances == nil {
		t.Fatal("FieldAffordances nil")
	}
	if w := (*got.FieldAffordances)["status"].Writable; w == nil || *w {
		t.Errorf("status.writable = %v, want false", w)
	}

	code, body := patchAs(t, app, "alice", "TKT-001", `{"properties":{"status":"closed"}}`)
	if code != http.StatusForbidden || !strings.Contains(body, `"field-affordance:read-only:status"`) {
		t.Fatalf("PATCH status = %d %s, want 403 field-affordance:read-only:status", code, body)
	}
	if strings.Contains(body, "outside_edits") {
		t.Errorf("wire body leaked the attribution: %s", body)
	}
	if code, body := patchAs(t, app, "alice", "TKT-001", `{"properties":{"title":"U"}}`); code != http.StatusOK {
		t.Errorf("PATCH title = %d %s, want 200", code, body)
	}
}

// TestFlowEdits_ProfileRefused: RELA_AFFORDANCE_PROFILE may not replace the
// resolver when a ceiling is wired, and a ceiling needs the policy.
func TestFlowEdits_ProfileRefused(t *testing.T) {
	app, d, ceiling := flowEditsApp(t)
	for _, profile := range []string{"none", "demo"} {
		t.Run(profile, func(t *testing.T) {
			_, err := ResolverFromProfile(profile, app.Meta(), app.store, d, ceiling)
			if !errors.Is(err, ErrProfileOverridesFieldRules) {
				t.Errorf("profile %s: err = %v, want ErrProfileOverridesFieldRules", profile, err)
			}
		})
	}
	t.Run("no policy", func(t *testing.T) {
		if _, err := ResolverFromProfile("", app.Meta(), app.store, nil, ceiling); err == nil {
			t.Error("a ceiling without acl.yaml was accepted")
		}
	})
}

// TestQueryBudget_ListDoesNoFlowRunLookup: a list page only redacts, and
// redaction does not depend on runs, so it looks no run up.
func TestQueryBudget_ListDoesNoFlowRunLookup(t *testing.T) {
	app, d, ceiling := flowEditsApp(t)
	for i := range 5 {
		seedEntity(app, &entity.Entity{ID: fmt.Sprintf("TKT-%03d", i), Type: "ticket",
			Properties: map[string]any{"title": "T", "status": "open"}})
	}
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/tickets", http.NoBody)
	rec := httptest.NewRecorder()
	app.handleV1ListEntities(rec, asAlice(t, d, req), "ticket", "tickets")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET list = %d %s", rec.Code, rec.Body)
	}
	if got := ceiling.lookups.Load(); got != 0 {
		t.Errorf("run lookups for a list page = %d, want 0", got)
	}
}

// TestQueryBudget_SectionFlowRunsSizeIndependent pins the collection-reads
// rule for the ceiling: an editable table section looks its rows' runs up
// once per section, not once per row.
func TestQueryBudget_SectionFlowRunsSizeIndependent(t *testing.T) {
	for _, n := range []int{5, 25} {
		t.Run(strconv.Itoa(n), func(t *testing.T) {
			app, d, ceiling := flowEditsApp(t)
			rows := make([]*entity.Entity, 0, n)
			for i := range n {
				e := &entity.Entity{ID: fmt.Sprintf("TKT-%03d", i), Type: "ticket",
					Properties: map[string]any{"title": "T", "status": "open"}}
				seedEntity(app, e)
				rows = append(rows, e)
			}
			sec := ViewSection{Heading: "Tickets", Source: "items", Display: "table",
				Columns: []ListColumn{{Property: "status", Render: dataentryconfig.RenderInput}}}
			out := app.views.buildSections(gateCtxFor(aliceCtx(), t, d), []ViewSection{sec},
				&viewResult{Collections: map[string][]*entity.Entity{"items": rows}})
			if len(out) != 1 || len(out[0].Rows) != n {
				t.Fatalf("sections = %+v, want one with %d rows", out, n)
			}
			for _, r := range out[0].Rows {
				if w := r.FieldAffordances["status"].Writable; w == nil || *w {
					t.Fatalf("row status.writable = %v, want false", w)
				}
			}
			if got := ceiling.lookups.Load(); got != 1 {
				t.Errorf("run lookups for %d rows = %d, want 1", n, got)
			}
		})
	}
}
