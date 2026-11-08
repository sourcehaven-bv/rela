package dataentry

import (
	"context"
	"log/slog"
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
	"github.com/Sourcehaven-BV/rela/internal/peersync"
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

// TestPeerPull_EndToEnd pulls from a real data-entry server through its
// peer key edge: the pull sees only what the peer's role reads, creates the
// rows and relations once, and applies an upstream change on the next run
// without touching a field the target added locally.
func TestPeerPull_EndToEnd(t *testing.T) {
	f := newPeerFixture(t, nil, nil)
	ctx := context.Background()
	if _, err := f.app.store.CreateRelation(ctx,
		entity.RelationKey{From: "TKT-001", Type: "blocks", To: "TKT-002"}, nil); err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(f.app.NewRouter())
	t.Cleanup(srv.Close)

	meta, err := metamodel.Parse([]byte(pullTargetSchema))
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := peersync.Parse([]byte(`
peers:
  acme:
    url: `+srv.URL+`
    credential: peer_credential.acme
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
	src, err := peersync.Dial(m.Peer, f.key, nil)
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
	deps := peersync.Deps{
		Source: src, Meta: meta, Writer: mgr, Store: st, State: &pullKV{},
		Logger: slog.New(slog.DiscardHandler), RunID: "e2e",
	}

	rep, err := peersync.Run(ctx, m, deps, peersync.Options{})
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
		t.Fatalf("a feature the peer cannot read was pulled: %v", got)
	}

	rep, err = peersync.Run(ctx, m, deps, peersync.Options{})
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
	rep, err = peersync.Run(ctx, m, deps, peersync.Options{})
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
