package dataentry

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/appbuild"
	"github.com/Sourcehaven-BV/rela/internal/appbuild/appbuildtest"
	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/entitymanager"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/project"
	"github.com/Sourcehaven-BV/rela/internal/storage"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/storetest"
)

// writeReadsMeta: alice's role reads tickets and features but not secrets.
// An on-create automation links every new ticket to SEC-1, a neighbor the
// creator cannot read.
const writeReadsMeta = `
entities:
  ticket:
    label: Ticket
    id_prefix: TKT
    properties:
      title: { type: string }
      note: { type: string }
  feature:
    label: Feature
    id_prefix: FEAT
    properties:
      title: { type: string }
  secret:
    label: Secret
    id_prefix: SEC
    properties:
      title: { type: string }
relations:
  implements:
    from: [ticket]
    to: [feature, secret]
automations:
  - name: link-secret
    on:
      entity: ticket
      created: true
    do:
      - create_relation:
          relation: implements
          to: SEC-1
`

// writeReadsApp builds an App whose store counts reads, with TKT-1 linked to
// neighbors visible features. role is alice's role; [ticketRole] is the
// usual one.
func writeReadsApp(
	t *testing.T, neighbors int, role acl.RoleDef,
) (*App, *acl.Declarative, *storetest.Counting, *audit.Memory) {
	t.Helper()
	meta, err := metamodel.Parse([]byte(writeReadsMeta))
	if err != nil {
		t.Fatalf("parse metamodel: %v", err)
	}
	fs := storage.NewMemFS()
	paths := &project.Context{Root: "/project", CacheDir: "/project/.rela"}
	if err = fs.MkdirAll(paths.CacheDir, 0o755); err != nil {
		t.Fatal(err)
	}
	st := storetest.NewCounting(appbuildtest.New(meta, appbuildtest.WithFS(fs, paths)).Store())
	ctx := context.Background()
	seeds := []*entity.Entity{
		{ID: "TKT-1", Type: "ticket", Properties: map[string]any{"title": "t", "note": "n"}},
		{ID: "SEC-1", Type: "secret", Properties: map[string]any{"title": "s"}},
		{ID: "FEAT-0", Type: "feature", Properties: map[string]any{"title": "f"}},
	}
	for i := range neighbors {
		seeds = append(seeds, &entity.Entity{ID: fmt.Sprintf("FEAT-%d", i+1), Type: "feature",
			Properties: map[string]any{"title": "f"}})
	}
	for _, e := range seeds {
		if err = st.CreateEntity(ctx, e); err != nil {
			t.Fatal(err)
		}
	}
	for i := range neighbors {
		k := entity.RelationKey{From: "TKT-1", Type: "implements", To: fmt.Sprintf("FEAT-%d", i+1)}
		if _, err = st.CreateRelation(ctx, k, &store.RelationData{}); err != nil {
			t.Fatal(err)
		}
	}

	d := mustNewACL(t, &acl.Policy{
		Roles:       map[string]acl.RoleDef{"r": role},
		Assignments: map[string]string{"alice": "r"},
	}, st)
	sink := &audit.Memory{}
	svc := appbuildtest.New(meta, appbuildtest.WithFS(fs, paths), appbuildtest.WithStore(st),
		appbuildtest.WithDeclarative(d), appbuildtest.WithAudit(sink))
	app := newAppFromParts(&Config{}, nil, newFixture())
	rebindApp(app, fs, paths, svc)
	app.acl = d
	app.schema.Publish(&Schema{Cfg: &Config{}, Meta: meta})
	if err = SetWorldNeighbors(app, st, appbuild.RelationScopes(svc)); err != nil {
		t.Fatal(err)
	}
	res, err := ResolverFromProfile("", meta, st, d)
	if err != nil {
		t.Fatal(err)
	}
	app.fieldResolver = res
	st.Reset()
	return app, d, st, sink
}

// ticketRole reads tickets and features and creates tickets. update says
// whether it may also update tickets.
func ticketRole(update bool) acl.RoleDef {
	role := acl.RoleDef{Read: []string{"ticket", "feature"}, Create: []string{"ticket"}}
	if update {
		role.Update = []string{"ticket"}
	}
	return role
}

// patchAsAlice sends a PATCH of TKT-1 as alice. A non-empty ifMatch is sent
// as the If-Match header.
func patchAsAlice(t *testing.T, app *App, d *acl.Declarative, body, ifMatch string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPatch, "/api/v1/tickets/TKT-1",
		strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if ifMatch != "" {
		req.Header.Set("If-Match", ifMatch)
	}
	rec := httptest.NewRecorder()
	app.write.handleV1UpdateEntity(rec, asAlice(t, d, req), "ticket", "tickets", "TKT-1")
	return rec
}

// getAsAlice sends a GET of TKT-1 as alice.
func getAsAlice(t *testing.T, app *App, d *acl.Declarative) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/tickets/TKT-1", nil)
	rec := httptest.NewRecorder()
	app.handleV1GetEntity(rec, asAlice(t, d, req), "ticket", "tickets", "TKT-1")
	return rec
}

// TestUpdate_ReadBudget pins the store cost of a PATCH. The response's edges
// are read once and serve both the body and the ETag, the target is loaded
// once by the resolver and once, raw, by the manager, and none of it grows
// with the number of neighbors. With If-Match, the check reads the edges one
// more time to compute the current ETag.
//
// One ListRelations is the identity walk asAlice runs at the request edge,
// as the production middleware does.
func TestUpdate_ReadBudget(t *testing.T) {
	for _, tc := range []struct {
		name          string
		ifMatch       bool
		wantRelations int
	}{
		{name: "no If-Match", wantRelations: 2},
		{name: "If-Match", ifMatch: true, wantRelations: 3},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var totals []int
			for _, n := range []int{10, 50} {
				app, d, st, _ := writeReadsApp(t, n, ticketRole(true))
				etag := ""
				if tc.ifMatch {
					etag = getAsAlice(t, app, d).Header().Get("ETag")
					st.Reset()
				}
				rec := patchAsAlice(t, app, d, `{"properties":{"title":"changed"}}`, etag)
				if rec.Code != http.StatusOK {
					t.Fatalf("n=%d: PATCH = %d %s", n, rec.Code, rec.Body)
				}
				got := st.Calls()
				if got["GetEntity"] != 2 {
					t.Errorf("n=%d: GetEntity = %d (%s), want 2: the resolver's gated read and the "+
						"manager's raw write-prep read", n, got["GetEntity"], st)
				}
				if got["ListRelations"] != tc.wantRelations {
					t.Errorf("n=%d: ListRelations = %d (%s), want %d: the identity walk, one edge read for "+
						"the response, and one for the If-Match check when the header is sent",
						n, got["ListRelations"], st, tc.wantRelations)
				}
				total := 0
				for _, v := range got {
					total += v
				}
				totals = append(totals, total)
			}
			if totals[0] != totals[1] {
				t.Errorf("PATCH cost grows with neighbors: %d reads at 10, %d at 50", totals[0], totals[1])
			}
		})
	}
}

// TestUpdate_DeniedWriteAuthorizesFirst pins that a PATCH the ACL refuses is
// refused after the resolver read and before any other work: one load, no
// edge read, no write-prep read, and exactly one `denied-write` audit row.
// A hidden target still answers 404 (TestACLWrite_PatchOnHiddenIs404).
func TestUpdate_DeniedWriteAuthorizesFirst(t *testing.T) {
	app, d, st, sink := writeReadsApp(t, 10, ticketRole(false))
	rec := patchAsAlice(t, app, d, `{"properties":{"title":"changed"}}`, "")
	if rec.Code != http.StatusForbidden {
		t.Fatalf("PATCH = %d %s, want 403", rec.Code, rec.Body)
	}
	// One GetEntity is the resolver's gated load. At most one relation query
	// is the ACL's own probe while it decides. No neighbor heads, no
	// edge page, and no raw write-prep read happen before the refusal.
	got := st.Calls()
	if got["GetEntity"] != 1 || got["ListRelations"] > 1 || got["ListEntityHeaders"] != 0 {
		t.Errorf("denied PATCH read %s, want one GetEntity, at most the ACL's one relation probe, "+
			"and no header reads", st)
	}
	denied := 0
	for _, r := range sink.Records() {
		if r.Op == audit.OpDeniedWrite {
			denied++
			if r.Subject == nil || r.Subject.ID != "TKT-1" || !strings.Contains(r.Summary, "attempted op=update") {
				t.Errorf("denied-write row = %+v, want the manager's shape for TKT-1", r)
			}
		}
	}
	if denied != 1 {
		t.Errorf("denied-write rows = %d, want 1", denied)
	}

	// A relations-only PATCH is not an entity update: the early ask does not
	// apply, so no entity update is asked or audited. The edge itself is
	// authorized by the manager as before.
	before := len(sink.Records())
	patchAsAlice(t, app, d, `{"relations":{"implements":{"data":[{"type":"feature","id":"FEAT-0"}]}}}`, "")
	for _, r := range sink.Records()[before:] {
		if r.Op == audit.OpDeniedWrite && r.Subject != nil && r.Subject.Kind == "entity" &&
			strings.Contains(r.Summary, "attempted op=update") {

			t.Errorf("relations-only PATCH was audited as a denied entity update: %+v", r)
		}
	}
}

// TestUpdate_RowDenialBeforeFieldVerdict pins the 403 a PATCH answers when
// the role may write the field but may not update the row. The row-level
// check runs first, so the caller gets the row-level denial, not the field
// affordance one.
func TestUpdate_RowDenialBeforeFieldVerdict(t *testing.T) {
	role := ticketRole(false)
	role.Fields = map[string][]acl.FieldGrant{"ticket": {{Field: "title"}}}
	app, d, _, _ := writeReadsApp(t, 0, role)
	rec := patchAsAlice(t, app, d, `{"properties":{"title":"changed"}}`, "")
	if rec.Code != http.StatusForbidden {
		t.Fatalf("PATCH = %d %s, want 403", rec.Code, rec.Body)
	}
	var body struct {
		RuleKind string `json:"rule_kind"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v (%s)", err, rec.Body)
	}
	if body.RuleKind == "affordance" || body.RuleKind == "" {
		t.Errorf("rule_kind = %q, want the row-level denial (%s)", body.RuleKind, rec.Body)
	}
}

// TestUpdate_ETagHidesHiddenFields pins that a PATCH response's ETag is the
// one a GET returns to the same caller, and that a value the caller cannot
// see does not move it. Hashing a hidden value would let the caller test
// guesses for it offline, and would fail the caller's next If-Match after a
// change it cannot see.
func TestUpdate_ETagHidesHiddenFields(t *testing.T) {
	role := ticketRole(true)
	role.Visible = map[string][]acl.FieldGrant{"ticket": {{Field: "title"}}}
	etagFor := func(note string) (patched, got string) {
		app, d, st, _ := writeReadsApp(t, 1, role)
		if err := st.UpdateEntity(t.Context(), &entity.Entity{ID: "TKT-1", Type: "ticket",
			Properties: map[string]any{"title": "t", "note": note}}); err != nil {
			t.Fatal(err)
		}
		rec := patchAsAlice(t, app, d, `{"properties":{"title":"changed"}}`, "")
		if rec.Code != http.StatusOK {
			t.Fatalf("PATCH = %d %s", rec.Code, rec.Body)
		}
		if strings.Contains(rec.Body.String(), note) {
			t.Fatalf("fixture: the PATCH response shows the hidden note: %s", rec.Body)
		}
		return rec.Header().Get("ETag"), getAsAlice(t, app, d).Header().Get("ETag")
	}
	patchedA, getA := etagFor("first-hidden-value")
	patchedB, _ := etagFor("second-hidden-value")
	if patchedA == "" || patchedA != getA {
		t.Errorf("PATCH ETag %q != GET ETag %q for the same caller and row", patchedA, getA)
	}
	if patchedA != patchedB {
		t.Errorf("PATCH ETag moved with a hidden value: %q vs %q", patchedA, patchedB)
	}
}

// managerHidesRow is a manager whose read gate refuses the PATCH target
// even though the resolver served it, as when a policy reload lands between
// the two reads. Only the early check is overridden.
type managerHidesRow struct{ entityMutator }

func (managerHidesRow) AuthorizeUpdate(_ context.Context, row *entity.Entity) error {
	return fmt.Errorf("%w: %s", entitymanager.ErrEntityNotFound, row.ID)
}

// TestUpdate_EarlyCheckHiddenRowIs404 pins that a principal who may update
// a row but not read it gets the uniform 404 from the early PATCH check,
// never the 403 that would confirm the row exists. The real manager's side
// is TestManager_AuthorizeUpdate; this pins the handler's mapping, and that
// the answer matches the one for an id that does not exist.
func TestUpdate_EarlyCheckHiddenRowIs404(t *testing.T) {
	app, d, _, sink := writeReadsApp(t, 0, ticketRole(true))
	app.write.manager = managerHidesRow{app.write.manager}
	rec := patchAsAlice(t, app, d, `{"properties":{"title":"changed"}}`, "")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("PATCH = %d %s, want 404", rec.Code, rec.Body)
	}
	missing := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPatch, "/api/v1/tickets/TKT-1",
		strings.NewReader(`{"properties":{"title":"changed"}}`))
	app.write.handleV1UpdateEntity(missing, asAlice(t, d, req), "ticket", "tickets", "TKT-404")
	if got, want := rec.Body.String(), missing.Body.String(); got != want {
		t.Errorf("hidden row answers %q, missing row %q", got, want)
	}
	for _, r := range sink.Records() {
		if r.Op == audit.OpDeniedWrite {
			t.Errorf("hidden row was audited as a denied write: %+v", r)
		}
	}
}
