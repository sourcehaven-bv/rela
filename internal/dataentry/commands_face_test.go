package dataentry

import (
	"context"
	"fmt"
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

			input := app.commands.buildEntityInput(ctx, publishedPOL1(ctx, t, app))

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

	input := app.commands.buildEntityInput(ctx, publishedPOL1(ctx, t, app))

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

	input := app.commands.buildViewInput(bg, "policy_view", vr)

	want := []string{
		"POL-1@ implements FEAT-1",
		"POL-1@ implements FEAT-DRAFT",
		"POL-1@published cites FEAT-PUB",
	}
	if got := edgeKeys(input.Relations); !slices.Equal(got, want) {
		t.Errorf("relations = %v, want %v", got, want)
	}
}

// The command payloads read their relations in a fixed number of store calls,
// however many edges or view entities there are (TKT-1U8XYN).
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

		if got := len(app.commands.buildEntityInput(ctx, e).Relations); got != n+2 {
			t.Fatalf("entity payload has %d relations, want %d", got, n+2)
		}
		entityCalls = counting.Reads()
		counting.Reset()
		if got := len(app.commands.buildViewInput(ctx, "v", vr).Relations); got != n+1 {
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
