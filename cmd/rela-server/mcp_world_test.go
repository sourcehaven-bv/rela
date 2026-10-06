//go:build !postgres

package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/appbuild"
	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/dataentry"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/project"
	"github.com/Sourcehaven-BV/rela/internal/script"
	"github.com/Sourcehaven-BV/rela/internal/search"
	"github.com/Sourcehaven-BV/rela/internal/storage"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/visibility"
)

// The shape of the BUG-6XTX0G report: a faced policy type whose entities have
// no default-face row, a faceless task type beside it, and a world that
// selects [adopted, concept]. `secret` is a type the role may not read.
const worldMetamodel = `version: "1.0"
entities:
  policy:
    label: Policy
    id_prefix: "POL-"
    id_type: sequential
    faces:
      concept: {}
      adopted: {}
    properties:
      title:
        type: string
  task:
    label: Task
    id_prefix: "TSK-"
    id_type: sequential
    properties:
      title:
        type: string
  secret:
    label: Secret
    id_prefix: "SEC-"
    id_type: sequential
    properties:
      title:
        type: string
relations:
  cites:
    from: [policy]
    to: [task]
    scope: content
worlds:
  current:
    select: [adopted, concept]
    otherwise: exclude
`

const worldPolicy = `roles:
  viewer:
    read: [policy, task, "world:current"]
  drafter:
    read: ["policy@concept", task, "world:current"]
role_relations:
  member-of:
    requires_permission: delegate-membership
assignments:
  alice: viewer
  carol: drafter
`

// newWorldServices builds services over worldMetamodel and worldPolicy and
// seeds POL-001 (adopted face only), TSK-001 and SEC-001, each titled with the
// word "retention" so one search can reach all three.
func newWorldServices(t *testing.T) *appbuild.Services {
	t.Helper()
	root := t.TempDir()
	for _, d := range []string{"entities", "relations", filepath.Join(".rela", "audit")} {
		if err := os.MkdirAll(filepath.Join(root, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for name, body := range map[string]string{"metamodel.yaml": worldMetamodel, "acl.yaml": worldPolicy} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	fs := storage.NewSafeFS(storage.NewOsFS())
	paths, err := project.Discover(root, fs)
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	svc, err := appbuild.New(appbuild.Config{
		FS: fs, Paths: paths, ScriptEngine: script.NewEngine(), Audit: audit.Nop{},
	})
	if err != nil {
		t.Fatalf("appbuild.New: %v", err)
	}
	t.Cleanup(func() { _ = svc.Close() })

	adopted, err := entity.ParseFace("adopted")
	if err != nil {
		t.Fatal(err)
	}
	for _, seed := range []struct {
		id, typ, title string
		face           entity.Face
	}{
		{"POL-001", "policy", "Retention policy", adopted},
		{"TSK-001", "task", "Retention task", ""},
		{"SEC-001", "secret", "Retention secret", ""},
	} {
		e := entity.New(seed.id, seed.typ)
		e.SetString("title", seed.title)
		e.Face = seed.face
		if err := svc.Store().CreateEntity(context.Background(), e); err != nil {
			t.Fatalf("seed %s: %v", seed.id, err)
		}
	}
	// POL-002 has both faces, and only its concept face cites TSK-001. The
	// world serves the adopted face, so that edge is not the adopted policy's.
	for _, face := range []string{"concept", "adopted"} {
		f, err := entity.ParseFace(face)
		if err != nil {
			t.Fatal(err)
		}
		e := entity.New("POL-002", "policy")
		e.SetString("title", "Archive policy")
		e.Face = f
		if err := svc.Store().CreateEntity(context.Background(), e); err != nil {
			t.Fatalf("seed POL-002@%s: %v", face, err)
		}
	}
	k := entity.RelationKey{From: "POL-002", FromFace: "concept", Type: "cites", To: "TSK-001"}
	if _, err := svc.Store().CreateRelation(context.Background(), k, nil); err != nil {
		t.Fatalf("seed concept-tailed edge: %v", err)
	}
	return svc
}

// testHost is an MCP host whose world functions select only `current`, the
// schema's one declared world.
func testHost(t *testing.T, svc *appbuild.Services) dataentry.MCPHost {
	t.Helper()
	compiled := appbuild.CompiledWorlds(svc)
	return dataentry.MCPHost{
		SelectWorld: func(_ context.Context, name string) (store.WorldScope, error) {
			scope, ok := compiled.Lookup(name)
			if !ok {
				return store.WorldScope{}, errors.New("no such world")
			}
			return scope, nil
		},
		WorldReadable: func(_ context.Context, name string) (bool, error) { return name == "current", nil },
		DefaultWorld:  compiled.DefaultWorldName,
	}
}

func aliceCtx() context.Context {
	return principal.With(context.Background(), principal.Principal{User: "alice", Tool: principal.ToolMCP})
}

func listIDs(t *testing.T, seq func(func(*entity.Entity, error) bool)) []string {
	t.Helper()
	var ids []string
	for e, err := range seq {
		if err != nil {
			t.Fatalf("list: %v", err)
		}
		ids = append(ids, entity.FormatStateRef(e.ID, e.Face))
	}
	slices.Sort(ids)
	return ids
}

func searchIDs(t *testing.T, svc *appbuild.Services, s search.Searcher) []string {
	t.Helper()
	var ids []string
	for h, err := range s.Search(aliceCtx(), search.Query{Text: "retention", World: appbuild.CompiledWorlds(svc).DefaultWorld()}) {
		if err != nil {
			t.Fatalf("search: %v", err)
		}
		ids = append(ids, entity.FormatStateRef(h.ID, h.Face))
	}
	slices.Sort(ids)
	return ids
}

// TestRemoteMCPDeps_FacedEntitiesResolveThroughTheWorld is the BUG-6XTX0G
// regression. Every remote MCP read handle, the store and the searcher, must
// serve a faced entity through the
// deployment's default world, serve an explicit ID@face literally, and still
// hide a type the role may not read.
func TestRemoteMCPDeps_FacedEntitiesResolveThroughTheWorld(t *testing.T) {
	svc := newWorldServices(t)
	deps, err := remoteMCPDeps(svc, testHost(t, svc))
	if err != nil {
		t.Fatalf("remoteMCPDeps: %v", err)
	}
	ctx := aliceCtx()

	readers := map[string]interface {
		Resolve(ctx context.Context, addr string) (*entity.Entity, error)
	}{
		"tools store": deps.Store,
	}
	for name, r := range readers {
		t.Run(name+" resolves a bare id", func(t *testing.T) {
			e, err := r.Resolve(ctx, "POL-001")
			if err != nil {
				t.Fatalf("GetEntity(POL-001) = %v, want the adopted face", err)
			}
			if e.Face.String() != "adopted" {
				t.Errorf("face = %q, want adopted", e.Face)
			}
		})
		t.Run(name+" serves an explicit face", func(t *testing.T) {
			if _, err := r.Resolve(ctx, "POL-001@adopted"); err != nil {
				t.Errorf("GetEntity(POL-001@adopted) = %v", err)
			}
		})
		t.Run(name+" hides an unreadable type", func(t *testing.T) {
			if _, err := r.Resolve(ctx, "SEC-001"); err == nil {
				t.Error("GetEntity(SEC-001) succeeded; the role may not read secret")
			}
		})
	}

	t.Run("tools store lists the faced type", func(t *testing.T) {
		got := listIDs(t, func(yield func(*entity.Entity, error) bool) {
			deps.Store.ListEntities(ctx, store.EntityQuery{Type: "policy", Faces: store.InWorld(deps.World)})(yield)
		})
		if want := []string{"POL-001@adopted", "POL-002@adopted"}; !slices.Equal(got, want) {
			t.Errorf("list policy = %v, want %v", got, want)
		}
	})
	searchers := map[string]search.Searcher{
		"tools searcher": deps.Searcher,
	}
	for name, s := range searchers {
		t.Run(name+" finds the faced entity and hides the unreadable one", func(t *testing.T) {
			want := []string{"POL-001@adopted", "TSK-001"}
			if got := searchIDs(t, svc, s); !slices.Equal(got, want) {
				t.Errorf("search = %v, want %v", got, want)
			}
		})
	}
}

// A reader granted only the concept face gets the concept face of an entity
// whose world prime is adopted: the gate filters the faces before the world
// ranks them, as on the data-entry entity GET. Ranking first would pick the
// adopted face and then hide it, so the entity would vanish.
func TestRemoteMCPDeps_FaceRestrictedReaderGetsTheFaceTheyMayRead(t *testing.T) {
	svc := newWorldServices(t)
	deps, err := remoteMCPDeps(svc, testHost(t, svc))
	if err != nil {
		t.Fatalf("remoteMCPDeps: %v", err)
	}
	carol := principal.With(context.Background(), principal.Principal{User: "carol", Tool: principal.ToolMCP})

	e, err := deps.Store.Resolve(carol, "POL-002")
	if err != nil {
		t.Fatalf("GetEntity(POL-002) = %v, want the concept face", err)
	}
	if e.Face.String() != "concept" {
		t.Errorf("face = %q, want concept", e.Face)
	}
	if _, err := deps.Store.Resolve(carol, "POL-001"); err == nil {
		t.Error("GetEntity(POL-001) succeeded; its only face is adopted, which carol may not read")
	}
	if _, err := deps.Store.Resolve(carol, "POL-002@adopted"); err == nil {
		t.Error("GetEntity(POL-002@adopted) succeeded; carol may not read the adopted face")
	}
}

func TestRemoteMCPDeps_RequiresTheHostWorldFunctions(t *testing.T) {
	svc := newWorldServices(t)
	if _, err := remoteMCPDeps(svc, dataentry.MCPHost{}); err == nil {
		t.Error("remoteMCPDeps accepted a host with no world functions; a world argument could not be authorized")
	}
}

// The remote server lets a tool name its world, through the host's selector,
// and a bare id read on the selected world's ctx resolves in that world.
func TestRemoteMCPDeps_PassesTheHostWorldSelector(t *testing.T) {
	svc := newWorldServices(t)
	deps, err := remoteMCPDeps(svc, testHost(t, svc))
	if err != nil {
		t.Fatalf("remoteMCPDeps: %v", err)
	}
	if deps.Worlds == nil {
		t.Fatal("remoteMCPDeps left Worlds nil; the world argument would be refused remotely")
	}
	scope, err := deps.Worlds.SelectWorld(aliceCtx(), "current")
	if err != nil || scope.IsTrivial() {
		t.Fatalf("SelectWorld(current) = %+v, %v; want the compiled world", scope, err)
	}
	if ok, _ := deps.Worlds.WorldReadable(context.Background(), "current"); !ok {
		t.Error("WorldReadable did not reach the host")
	}
	if got := deps.Worlds.DefaultWorld(); got != "current" {
		t.Errorf("DefaultWorld = %q, want current", got)
	}

	// The trivial world selects no named face, so a bare id of a type whose
	// rows are all faced resolves to nothing there.
	trivial := visibility.WithReadWorld(aliceCtx(), visibility.WorldOf(store.TrivialScope()))
	if _, err := deps.Store.Resolve(trivial, "POL-001"); err == nil {
		t.Error("Resolve(POL-001) in the trivial world found a face; the ctx world was ignored")
	}
	if _, err := deps.Store.Resolve(visibility.WithReadWorld(aliceCtx(), visibility.WorldOf(scope)), "POL-001"); err != nil {
		t.Errorf("Resolve(POL-001) in current = %v", err)
	}
}
