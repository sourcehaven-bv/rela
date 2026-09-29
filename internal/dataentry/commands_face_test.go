package dataentry

import (
	"context"
	"errors"
	"fmt"
	"iter"
	"slices"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/storetest"
)

// A command's stdin is piped to an operator script, past any layer that could
// observe it, so its relations follow the same two rules as every read-out
// surface (BUG-BZQQDP, TKT-2FDTJE): a content-scoped edge travels only with
// the face that owns it, and an edge travels only when both of its endpoints
// are readable.

// edgeKeys renders rels as "from@face type to" for order-free comparison.
func edgeKeys(rels []*entity.Relation) []string {
	out := make([]string, 0, len(rels))
	for _, r := range rels {
		out = append(out, fmt.Sprintf("%s@%s %s %s", r.From, r.FromFace, r.Type, r.To))
	}
	slices.Sort(out)
	return out
}

// publishedPOL1 reads POL-1's published face as the gate in ctx sees it.
func publishedPOL1(ctx context.Context, t *testing.T, app *App) *entity.Entity {
	t.Helper()
	e, ok, err := app.visibleReader.address(ctx, "policy", "POL-1@published")
	if err != nil || !ok {
		t.Fatalf("read POL-1@published: ok=%v err=%v", ok, err)
	}
	return e
}

func TestCommandEntityInput_ContentEdgesOnlyWithTheirFace(t *testing.T) {
	for _, reader := range contentEdgeReaders {
		t.Run(reader.name, func(t *testing.T) {
			app, d := contentEdgeApp(t, reader)
			ctx := gateCtxFor(aliceCtx(), t, d)

			input := mustEntityInput(ctx, t, app, publishedPOL1(ctx, t, app))

			want := []string{"POL-1@ implements FEAT-1", "POL-1@published cites FEAT-PUB"}
			if got := edgeKeys(input.Relations); !slices.Equal(got, want) {
				t.Errorf("relations = %v, want %v (never the draft face's cites edge)", got, want)
			}
		})
	}
}

// A peer the principal may not read never reaches the script, whichever
// direction the edge points and whichever scope it has.
func TestCommandEntityInput_PeerGated(t *testing.T) {
	app, d := facedApp(t, func(st store.Store) *acl.Declarative {
		return mustNewACL(t, &acl.Policy{
			Roles:       map[string]acl.RoleDef{"reader": {Read: []string{"policy@published"}}},
			Assignments: map[string]string{"alice": "reader"},
		}, st)
	})
	bg := context.Background()
	if err := app.store.CreateEntity(bg, &entity.Entity{
		ID: "POL-2", Type: "policy", Face: "draft", Properties: map[string]any{"title": "hidden draft"},
	}); err != nil {
		t.Fatal(err)
	}
	for _, r := range []struct {
		from, typ, to string
		tail          entity.Face
	}{
		{"POL-1", "implements", "FEAT-1", ""},         // hidden head
		{"FEAT-1", "governs", "POL-1", ""},            // hidden tail, incoming
		{"POL-2", "relates-to", "POL-1", "draft"},     // tail on a face alice cannot read
		{"POL-1", "relates-to", "POL-1", "published"}, // readable self-edge
	} {
		if _, err := app.store.CreateRelation(bg, r.from, r.typ, r.to,
			&store.RelationData{FromFace: r.tail}); err != nil {
			t.Fatalf("seed %s %s %s: %v", r.from, r.typ, r.to, err)
		}
	}
	ctx := gateCtxFor(aliceCtx(), t, d)

	input := mustEntityInput(ctx, t, app, publishedPOL1(ctx, t, app))

	want := []string{"POL-1@published relates-to POL-1"}
	if got := edgeKeys(input.Relations); !slices.Equal(got, want) {
		t.Errorf("LEAK: relations = %v, want %v", got, want)
	}
}

// The view payload's edges come from ONE relation query and are kept only
// when the face the view serves their source at owns them. POL-1's draft
// `cites` edge ends at FEAT-DRAFT, which the view collects over an identity
// edge, so only the face check keeps the draft edge out.
func TestCommandViewInput_ContentEdgesOnlyWithTheirFace(t *testing.T) {
	app, _ := contentEdgeApp(t, contentEdgeReaders[0])
	bg := context.Background()
	if _, err := app.store.CreateRelation(bg, "POL-1", "implements", "FEAT-DRAFT", nil); err != nil {
		t.Fatal(err)
	}
	view := ViewConfig{
		Entry: ViewEntry{Type: "policy"},
		Traverse: []ViewTraverse{
			{From: "entry", Follow: "cites", CollectAs: "cited"},
			{From: "entry", Follow: "implements", CollectAs: "implemented"},
		},
	}
	vr, err := app.views.executeView(bg, view, "POL-1@published", defaultViewWorld())
	if err != nil {
		t.Fatalf("executeView: %v", err)
	}

	input := mustViewInput(bg, t, app, "policy_view", vr)

	want := []string{
		"POL-1@ implements FEAT-1",
		"POL-1@ implements FEAT-DRAFT",
		"POL-1@published cites FEAT-PUB",
	}
	if got := edgeKeys(input.Relations); !slices.Equal(got, want) {
		t.Errorf("relations = %v, want %v", got, want)
	}
}

// relFailingStore fails every relation listing.
type relFailingStore struct{ store.Store }

func (relFailingStore) ListRelations(context.Context, store.RelationQuery) iter.Seq2[*entity.Relation, error] {
	return func(yield func(*entity.Relation, error) bool) { yield(nil, errors.New("relation read failed")) }
}

// A failed relation or header read fails the payload build, so the command
// never runs on stdin that claims the entity has no edges.
func TestCommandInputs_ReadFaultFailsTheBuild(t *testing.T) {
	for _, tc := range []struct {
		name      string
		wrap      func(store.Store) store.Store
		viewFails bool // the view payload has no header read, so only a relation fault fails it
	}{
		{"relation read", func(s store.Store) store.Store { return relFailingStore{s} }, true},
		{"endpoint header read", func(s store.Store) store.Store { return listFailingStore{s} }, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			app, d := contentEdgeApp(t, contentEdgeReaders[0])
			ctx := gateCtxFor(aliceCtx(), t, d)
			e := publishedPOL1(ctx, t, app)
			vr, err := app.views.executeView(ctx, ViewConfig{
				Entry:    ViewEntry{Type: "policy"},
				Traverse: []ViewTraverse{{From: "entry", Follow: "implements", CollectAs: "implemented"}},
			}, "POL-1@published", defaultViewWorld())
			if err != nil {
				t.Fatalf("executeView: %v", err)
			}
			rebindCommandStore(t, app, tc.wrap(app.store))
			meta := app.schema.Current().Meta

			if in, err := app.commands.buildEntityInput(ctx, meta, e); err == nil {
				t.Errorf("entity payload built despite a failed read: %d relations", len(in.Relations))
			}
			if _, err := app.commands.buildViewInput(ctx, meta, "v", vr); (err != nil) != tc.viewFails {
				t.Errorf("view payload err = %v, want failure %v", err, tc.viewFails)
			}
		})
	}
}

// One id served at two faces keeps the content edges of both faces and no
// other face's.
func TestCommandViewInput_IDAtTwoFaces(t *testing.T) {
	app, _ := contentEdgeApp(t, contentEdgeReaders[0])
	bg := context.Background()
	if _, err := app.store.CreateRelation(bg, "POL-1", "implements", "FEAT-DRAFT", nil); err != nil {
		t.Fatal(err)
	}
	row := func(id string, face entity.Face) *entity.Entity {
		return &entity.Entity{ID: id, Type: "policy", Face: face}
	}
	feat := func(id string) *entity.Entity { return &entity.Entity{ID: id, Type: "feature"} }
	base := &viewResult{
		Entry: row("POL-1", "published"),
		Collections: map[string][]*entity.Entity{
			"features": {feat("FEAT-DRAFT"), feat("FEAT-PUB"), feat("FEAT-1")},
		},
	}
	both := &viewResult{Entry: base.Entry, Collections: map[string][]*entity.Entity{
		"features": base.Collections["features"],
		"faces":    {row("POL-1", "draft")},
	}}

	for _, tc := range []struct {
		name string
		vr   *viewResult
		want []string
	}{
		{"published only", base, []string{
			"POL-1@ implements FEAT-1", "POL-1@ implements FEAT-DRAFT", "POL-1@published cites FEAT-PUB",
		}},
		{"draft and published", both, []string{
			"POL-1@ implements FEAT-1", "POL-1@ implements FEAT-DRAFT",
			"POL-1@draft cites FEAT-DRAFT", "POL-1@published cites FEAT-PUB",
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := edgeKeys(mustViewInput(bg, t, app, "v", tc.vr).Relations); !slices.Equal(got, tc.want) {
				t.Errorf("relations = %v, want %v", got, tc.want)
			}
		})
	}
}

// The command payloads read their relations in a fixed number of store calls,
// however many edges or view entities there are (TKT-1U8XYN). The budget
// counts store reads made by the payload builders; the ACL gate is built over
// the uncounted store, so its own membership reads are not in it.
func TestCommandInputs_RelationReadBudget(t *testing.T) {
	calls := func(t *testing.T, n int) (entityCalls, viewCalls int) {
		t.Helper()
		app, d := contentEdgeApp(t, contentEdgeReaders[0])
		bg := context.Background()
		for i := range n {
			id := fmt.Sprintf("FEAT-N%d", i)
			if err := app.store.CreateEntity(bg, &entity.Entity{
				ID: id, Type: "feature", Properties: map[string]any{"title": id},
			}); err != nil {
				t.Fatal(err)
			}
			if _, err := app.store.CreateRelation(bg, "POL-1", "implements", id, nil); err != nil {
				t.Fatal(err)
			}
		}
		ctx := gateCtxFor(aliceCtx(), t, d)
		e := publishedPOL1(ctx, t, app)
		view := ViewConfig{
			Entry:    ViewEntry{Type: "policy"},
			Traverse: []ViewTraverse{{From: "entry", Follow: "implements", CollectAs: "implemented"}},
		}
		vr, err := app.views.executeView(ctx, view, "POL-1@published", defaultViewWorld())
		if err != nil {
			t.Fatalf("executeView: %v", err)
		}
		if got := len(vr.Collections["implemented"]); got != n+1 {
			t.Fatalf("view collected %d, want %d", got, n+1)
		}

		counting := storetest.NewCounting(app.store)
		rebindCommandStore(t, app, counting)

		if got := len(mustEntityInput(ctx, t, app, e).Relations); got != n+2 {
			t.Fatalf("entity payload has %d relations, want %d", got, n+2)
		}
		entityCalls = counting.Reads()
		counting.Reset()
		if got := len(mustViewInput(ctx, t, app, "v", vr).Relations); got != n+1 {
			t.Fatalf("view payload has %d relations, want %d", got, n+1)
		}
		return entityCalls, counting.Reads()
	}
	e10, v10 := calls(t, 10)
	e50, v50 := calls(t, 50)
	if e10 != e50 {
		t.Errorf("entity payload reads grow with edges: %d at 10, %d at 50", e10, e50)
	}
	if v10 != v50 {
		t.Errorf("view payload reads grow with entities: %d at 10, %d at 50", v10, v50)
	}
}

// rebindCommandStore points the command handler's relation read and its
// resolver at st, so a counting wrapper sees every read the payload makes.
func rebindCommandStore(t *testing.T, app *App, st store.Store) {
	t.Helper()
	vr, err := newVisibleReader(st)
	if err != nil {
		t.Fatal(err)
	}
	orig := app.commands.services
	app.commands.services = func() Services {
		svc := orig()
		svc.Store = st
		return svc
	}
	app.commands.visible = vr
}

// mustEntityInput builds an entity-context payload under the app's current
// schema and fails the test on a read error.
func mustEntityInput(ctx context.Context, t *testing.T, app *App, e *entity.Entity) *commandInput {
	t.Helper()
	in, err := app.commands.buildEntityInput(ctx, app.schema.Current().Meta, e)
	if err != nil {
		t.Fatalf("buildEntityInput: %v", err)
	}
	return in
}

// mustViewInput is [mustEntityInput] for a view-context payload.
func mustViewInput(ctx context.Context, t *testing.T, app *App, viewID string, vr *viewResult) *commandInput {
	t.Helper()
	in, err := app.commands.buildViewInput(ctx, app.schema.Current().Meta, viewID, vr)
	if err != nil {
		t.Fatalf("buildViewInput: %v", err)
	}
	return in
}
