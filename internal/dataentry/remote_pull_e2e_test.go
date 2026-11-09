package dataentry

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"slices"
	"sync"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/entitymanager"
	"github.com/Sourcehaven-BV/rela/internal/lock"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/remotesync"
	"github.com/Sourcehaven-BV/rela/internal/statemachine"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
	"github.com/Sourcehaven-BV/rela/internal/templating"
)

const pullTargetSchema = `
version: "1.0"
entities:
  item:
    label: Item
    id_prefix: "ITM-"
    properties:
      title: {type: string}
      note: {type: string}
      src: {type: external_ref, system: source}
  thing:
    label: Thing
    id_prefix: "THG-"
    properties:
      title: {type: string}
      src: {type: external_ref, system: source}
relations:
  blocks:
    label: Blocks
    from: [item]
    to: [item]
`

// pullToken is the credential the identity proxy in front of the source
// accepts for the pulling instance.
const pullToken = "machine-client-token"

// pullSource is a data-entry server behind an identity proxy, as in a
// header-mode deployment. The proxy turns pullToken into the principal
// header for catalog-mirror, an ordinary principal whose role reads tickets
// only. Any other request reaches the server without an identity.
type pullSource struct {
	app *App
	url string
}

func newPullSource(t *testing.T) *pullSource {
	t.Helper()
	app := newTestAppV1(t)
	for _, e := range []*entity.Entity{
		{ID: "TKT-001", Type: "ticket", Properties: map[string]any{"title": "one"}},
		{ID: "TKT-002", Type: "ticket", Properties: map[string]any{"title": "two"}},
		{ID: "FEAT-001", Type: "feature", Properties: map[string]any{"title": "f1"}},
	} {
		seedEntity(app, e)
	}
	d := mustNewACL(t, &acl.Policy{
		Roles: map[string]acl.RoleDef{
			"tickets-reader": {Read: []string{"ticket"}},
		},
		Assignments: map[string]string{"catalog-mirror": "tickets-reader"},
	}, app.store)
	setTestACL(t, app, d)
	if err := SetReconcile(app, localElements(t, app, d)); err != nil {
		t.Fatal(err)
	}
	const header = "X-Remote-User"
	app.SetPrincipalResolver(HeaderPrincipalResolver(header))
	app.SetPrincipalHeader(header)
	router := app.NewRouter()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Header.Del(header)
		if r.Header.Get("Authorization") == "Bearer "+pullToken {
			r.Header.Set(header, "catalog-mirror")
		}
		r.Header.Del("Authorization")
		router.ServeHTTP(w, r)
	}))
	t.Cleanup(srv.Close)
	return &pullSource{app: app, url: srv.URL}
}

// TestRemotePull_EndToEnd pulls from a real data-entry server as an
// ordinary principal: the pull sees only what that principal's role reads,
// creates the rows and relations once, and applies an upstream change on
// the next run without touching a field the target added locally.
func TestRemotePull_EndToEnd(t *testing.T) {
	f := newPullSource(t)
	ctx := context.Background()
	if _, err := f.app.store.CreateRelation(ctx,
		entity.RelationKey{From: "TKT-001", Type: "blocks", To: "TKT-002"}, nil); err != nil {
		t.Fatal(err)
	}

	meta, err := metamodel.Parse([]byte(pullTargetSchema))
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := remotesync.Parse([]byte(`
remotes:
  acme:
    url: `+f.url+`
    credential: {secret: remote_credential.acme}
    entities:
      ticket: {type: item, ref: src, fields: {title: title}}
      feature: {type: thing, ref: src, fields: {title: title}}
    relations:
      blocks: {}
`), meta)
	if err != nil {
		t.Fatal(err)
	}
	m := cfg.Mappings["acme"]
	credential, err := m.Remote.ReadCredential(nil, func(name string) (string, error) {
		return map[string]string{"remote_credential.acme": pullToken}[name], nil
	})
	if err != nil {
		t.Fatal(err)
	}
	src, err := remotesync.Dial(m.Remote, credential, nil)
	if err != nil {
		t.Fatal(err)
	}
	st := memstore.New()
	mgr, err := entitymanager.New(entitymanager.Deps{
		Store: st, Meta: meta, Templater: pullTemplater{}, Audit: audit.Nop{}, ACL: acl.NopACL{},
		Transitions: statemachine.EmptySet(), FieldGate: entitymanager.AllowAllFieldGate{},
		AttachmentLocker: lock.NewMemoryLocker(), AnchorGuard: entitymanager.NopAnchorGuard{},
	})
	if err != nil {
		t.Fatal(err)
	}
	deps := remotesync.Deps{
		Source: src, Meta: meta, Writer: mgr, Store: st, State: &pullKV{},
		Logger: slog.New(slog.DiscardHandler), RunID: "e2e",
	}

	rep, err := remotesync.Run(ctx, m, deps, remotesync.Options{})
	if err != nil {
		t.Fatalf("first pull: %v (%+v)", err, rep)
	}
	if rep.Entities.Created != 2 || rep.Relations.Created != 1 {
		t.Fatalf("want the 2 tickets and their relation: %+v", rep)
	}
	titles := pulledTitles(t, st, "item")
	if !slices.Equal(titles, []string{"one", "two"}) {
		t.Fatalf("items = %v", titles)
	}
	if got := pulledTitles(t, st, "thing"); len(got) != 0 {
		t.Fatalf("a feature the remote cannot read was pulled: %v", got)
	}

	rep, err = remotesync.Run(ctx, m, deps, remotesync.Options{})
	if err != nil || rep.Entities.Created != 0 || rep.Entities.Updated != 0 {
		t.Fatalf("second pull is not idempotent: %v %+v", err, rep)
	}

	local := pulledByTitle(t, st, "one")
	if _, err = mgr.PatchEntity(ctx, local.ID, entity.Patch{
		Properties: map[string]any{"note": "local addition"},
	}); err != nil {
		t.Fatal(err)
	}

	e, err := f.app.store.GetEntity(ctx, entity.Ref{ID: "TKT-001"})
	if err != nil {
		t.Fatal(err)
	}
	e.Properties["title"] = "one, renamed"
	if err = f.app.store.UpdateEntity(ctx, e); err != nil {
		t.Fatal(err)
	}
	rep, err = remotesync.Run(ctx, m, deps, remotesync.Options{})
	if err != nil || rep.Entities.Updated != 1 {
		t.Fatalf("third pull: %v %+v", err, rep)
	}
	if titles := pulledTitles(t, st, "item"); !slices.Equal(titles, []string{"one, renamed", "two"}) {
		t.Fatalf("items after the change = %v", titles)
	}
	if got := pulledByTitle(t, st, "one, renamed"); got.ID != local.ID || got.Properties["note"] != "local addition" {
		t.Fatalf("the upstream update lost the local addition: %+v", got)
	}
	rels := 0
	for _, err := range st.ListRelations(ctx, store.RelationQuery{Type: "blocks"}) {
		if err != nil {
			t.Fatal(err)
		}
		rels++
	}
	if rels != 1 {
		t.Fatalf("blocks relations = %d, want 1", rels)
	}

	// A token the proxy no longer accepts reaches the source without an
	// identity, which reads nothing. That must not read as every row gone.
	stale, err := remotesync.Dial(m.Remote, "revoked-token", nil)
	if err != nil {
		t.Fatal(err)
	}
	deps.Source = stale
	rep, err = remotesync.Run(ctx, m, deps, remotesync.Options{})
	if err == nil || rep.Entities.Detached != 0 || rep.Entities.Deleted != 0 {
		t.Fatalf("a pull without an identity was applied: %v %+v", err, rep)
	}
	if titles := pulledTitles(t, st, "item"); len(titles) != 2 {
		t.Fatalf("items after a pull without an identity = %v", titles)
	}
}

func pulledByTitle(t *testing.T, st store.Store, title string) *entity.Entity {
	t.Helper()
	for e, err := range st.ListEntities(context.Background(), store.EntityQuery{Type: "item", Faces: store.AllFaces()}) {
		if err != nil {
			t.Fatal(err)
		}
		if e.Properties["title"] == title {
			return e
		}
	}
	t.Fatalf("no item titled %q", title)
	return nil
}

func pulledTitles(t *testing.T, st store.Store, typ string) []string {
	t.Helper()
	var out []string
	for e, err := range st.ListEntities(context.Background(), store.EntityQuery{Type: typ, Faces: store.AllFaces()}) {
		if err != nil {
			t.Fatal(err)
		}
		title, _ := e.Properties["title"].(string)
		out = append(out, title)
	}
	slices.Sort(out)
	return out
}

// pullTemplater has no templates.
type pullTemplater struct{}

func (pullTemplater) EntityTemplate(context.Context, string, string) (*templating.Template, error) {
	return nil, nil //nolint:nilnil // no template
}

func (pullTemplater) RelationTemplate(context.Context, string) (*templating.Template, error) {
	return nil, nil //nolint:nilnil // no template
}

// pullKV is an in-memory state.KV for the pull's ledger.
type pullKV struct {
	mu sync.Mutex
	m  map[string][]byte
}

func (k *pullKV) Get(_ context.Context, key string) ([]byte, error) {
	k.mu.Lock()
	defer k.mu.Unlock()
	v, ok := k.m[key]
	if !ok {
		return nil, os.ErrNotExist
	}
	return v, nil
}

func (k *pullKV) Put(_ context.Context, key string, data []byte) error {
	k.mu.Lock()
	defer k.mu.Unlock()
	if k.m == nil {
		k.m = map[string][]byte{}
	}
	k.m[key] = slices.Clone(data)
	return nil
}

func (k *pullKV) Delete(_ context.Context, key string) error {
	k.mu.Lock()
	defer k.mu.Unlock()
	delete(k.m, key)
	return nil
}
