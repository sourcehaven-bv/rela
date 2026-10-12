package dataentry

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/appbuild"
	"github.com/Sourcehaven-BV/rela/internal/appbuild/appbuildtest"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/project"
	"github.com/Sourcehaven-BV/rela/internal/storage"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// Write responses render the written entity's edges. They must read them
// through the same neighbor gate as the GET, so a response never names a
// neighbor the caller cannot read (BUG-J01V0Q).

// hiddenNeighborMeta: alice reads tickets and features but not secrets. An
// on-create automation links every new ticket to SEC-1.
const hiddenNeighborMeta = `
entities:
  ticket:
    label: Ticket
    id_prefix: TKT
    properties:
      title: { type: string }
      report: { type: file }
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
attachments:
  allow: [text/plain]
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

// hiddenNeighborApp builds an App with TKT-1 linked to the visible FEAT-0
// and to SEC-1, which alice cannot read.
func hiddenNeighborApp(t *testing.T) (*App, *acl.Declarative, store.Store) {
	t.Helper()
	meta, err := metamodel.Parse([]byte(hiddenNeighborMeta))
	if err != nil {
		t.Fatalf("parse metamodel: %v", err)
	}
	fs := storage.NewMemFS()
	paths := &project.Context{Root: "/project", CacheDir: "/project/.rela"}
	if err := fs.MkdirAll(paths.CacheDir, 0o755); err != nil {
		t.Fatal(err)
	}
	st := appbuildtest.New(meta, appbuildtest.WithFS(fs, paths)).Store()
	ctx := context.Background()
	for _, e := range []*entity.Entity{
		{ID: "TKT-1", Type: "ticket", Properties: map[string]any{"title": "t"}},
		{ID: "SEC-1", Type: "secret", Properties: map[string]any{"title": "CLASSIFIED-TITLE"}},
		{ID: "FEAT-0", Type: "feature", Properties: map[string]any{"title": "f"}},
	} {
		if err := st.CreateEntity(ctx, e); err != nil {
			t.Fatal(err)
		}
	}
	for _, to := range []string{"FEAT-0", "SEC-1"} {
		k := entity.RelationKey{From: "TKT-1", Type: "implements", To: to}
		if _, err := st.CreateRelation(ctx, k, &store.RelationData{}); err != nil {
			t.Fatal(err)
		}
	}

	d := mustNewACL(t, &acl.Policy{
		Roles: map[string]acl.RoleDef{"r": {
			Read:        []string{"ticket", "feature"},
			Create:      []string{"ticket"},
			Update:      []string{"ticket"},
			Permissions: []string{string(acl.PermHistoryRead)},
		}},
		Assignments: map[string]string{"alice": "r"},
	}, st)
	svc := appbuildtest.New(meta, appbuildtest.WithFS(fs, paths), appbuildtest.WithStore(st),
		appbuildtest.WithDeclarative(d))
	app := newAppFromParts(&Config{}, nil, newFixture())
	rebindApp(app, fs, paths, svc)
	app.acl = d
	app.schema.Publish(&Schema{Cfg: &Config{}, Meta: meta})
	if err := SetWorldNeighbors(app, st, appbuild.RelationScopes(svc)); err != nil {
		t.Fatal(err)
	}
	return app, d, st
}

// assertHidesSecret fails when body names SEC-1 or its title, and checks the
// visible neighbor is kept.
func assertHidesSecret(t *testing.T, body []byte, visible string) {
	t.Helper()
	if s := string(body); strings.Contains(s, "SEC-1") || strings.Contains(s, "CLASSIFIED-TITLE") {
		t.Errorf("response names a neighbor alice cannot read: %s", s)
	}
	if visible == "" {
		return
	}
	var wire struct {
		Relations map[string][]string `json:"relations"`
	}
	if err := json.Unmarshal(body, &wire); err != nil {
		t.Fatalf("decode: %v (%s)", err, body)
	}
	if !containsID(wire.Relations["implements"], visible) {
		t.Errorf("relations[implements] = %v, want the visible %s kept", wire.Relations["implements"], visible)
	}
}

func TestCreate_ResponseHidesUnreadableNeighbor(t *testing.T) {
	app, d, st := hiddenNeighborApp(t)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/v1/tickets", strings.NewReader(
		`{"properties":{"title":"new"},`+
			`"relations":{"implements":{"data":[{"type":"feature","id":"FEAT-0"}]}}}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	app.write.handleV1CreateEntity(rec, asAlice(t, d, req), "ticket", "tickets")
	if rec.Code != http.StatusCreated {
		t.Fatalf("POST = %d %s", rec.Code, rec.Body)
	}
	var body struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v (%s)", err, rec.Body)
	}

	// The automation did link the secret; otherwise the test proves nothing.
	linked := false
	for rel, err := range st.ListRelations(t.Context(), store.RelationQuery{From: body.ID}) {
		if err != nil {
			t.Fatal(err)
		}
		linked = linked || rel.To == "SEC-1"
	}
	if !linked {
		t.Fatal("fixture: the on-create automation did not link SEC-1")
	}
	assertHidesSecret(t, rec.Body.Bytes(), "FEAT-0")
}

func TestAttachmentUpload_ResponseHidesUnreadableNeighbor(t *testing.T) {
	app, d, _ := hiddenNeighborApp(t)
	alice := principal.With(t.Context(), principal.Principal{User: "alice", Tool: principal.ToolDataEntry})
	rec := putAttachmentAs(alice, t, app, d, "TKT-1", "report", "a.txt", []byte("hello"))
	if rec.Code != http.StatusOK {
		t.Fatalf("upload = %d %s", rec.Code, rec.Body)
	}
	assertHidesSecret(t, rec.Body.Bytes(), "FEAT-0")
}

func TestHistoryRestore_ResponseHidesUnreadableNeighbor(t *testing.T) {
	app, d, _ := hiddenNeighborApp(t)
	app.versions = historyStore{versions: map[string][]store.VersionSnapshot{
		"TKT-1": {snapshot("ticket", "old body", map[string]any{"title": "then"})},
	}}
	alice := principal.With(t.Context(), principal.Principal{User: "alice", Tool: principal.ToolDataEntry})
	rec := historyAs(alice, t, app, d, http.MethodPost, "TKT-1/1/restore")
	if rec.Code != http.StatusOK {
		t.Fatalf("restore = %d %s", rec.Code, rec.Body)
	}
	var body struct {
		Entity json.RawMessage `json:"entity"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v (%s)", err, rec.Body)
	}
	assertHidesSecret(t, rec.Body.Bytes(), "")
	assertHidesSecret(t, body.Entity, "FEAT-0")
}

// TestServedFaceEdges_UnwiredFailsClosed: without the neighbor gate the edge
// read errors rather than returning ungated edges a response would serialize.
func TestServedFaceEdges_UnwiredFailsClosed(t *testing.T) {
	e := &entity.Entity{ID: "TKT-1", Type: "ticket"}
	rels, visible, err := servedFaceEdges(t.Context(), nil, e)
	if !errors.Is(err, errNeighborsUnwired) || rels != nil || visible != nil {
		t.Fatalf("servedFaceEdges(nil wiring) = %v, %v, %v; want errNeighborsUnwired", rels, visible, err)
	}
}
