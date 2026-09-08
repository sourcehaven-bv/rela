package appbuild_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/appbuild"
	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/project"
	"github.com/Sourcehaven-BV/rela/internal/script"
	"github.com/Sourcehaven-BV/rela/internal/search"
	"github.com/Sourcehaven-BV/rela/internal/storage"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
)

// newSharedBase builds a SharedBase over a minimal on-disk project.
func newSharedBase(t *testing.T) *appbuild.SharedBase {
	t.Helper()
	root := writeMinimalProject(t)
	fs := storage.NewSafeFS(storage.NewOsFS())
	paths, err := project.Discover(root, fs)
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	base, err := appbuild.NewSharedBase(appbuild.Config{
		FS:           fs,
		Paths:        paths,
		ScriptEngine: script.NewEngine(),
		Audit:        audit.Nop{},
	})
	if err != nil {
		t.Fatalf("NewSharedBase: %v", err)
	}
	return base
}

// assembleOver wires base against a fresh in-memory store, the way a
// multi-store host would (one base, one Assemble per store).
func assembleOver(t *testing.T, base *appbuild.SharedBase) *appbuild.Services {
	t.Helper()
	st := memstore.New()
	searcher := search.New(st, search.NewLinearSearch())
	svc, err := base.Assemble(st, searcher, nil, nil)
	if err != nil {
		t.Fatalf("Assemble: %v", err)
	}
	return svc
}

// TestSharedBase_AssembleTwiceOverDistinctStores is the property this refactor
// exists for: one parsed configuration, several independent stores.
//
// It asserts the stores are genuinely separate (a write through one is not
// visible through the other) AND that both bundles share the exact metamodel
// and ACL policy faces from the base — i.e. the config was parsed once, not
// re-read per store. Without both halves the test would pass against a
// build that quietly re-prepared per assembly.
func TestSharedBase_AssembleTwiceOverDistinctStores(t *testing.T) {
	base := newSharedBase(t)

	a := assembleOver(t, base)
	t.Cleanup(func() { _ = a.Close() })
	b := assembleOver(t, base)
	t.Cleanup(func() { _ = b.Close() })

	// Shared, parsed once: identical faces, not merely equal values.
	if a.Meta() != base.Meta() || b.Meta() != base.Meta() {
		t.Error("assembled Services must reuse the base's metamodel, not reload it")
	}
	if a.ACLPolicy() != b.ACLPolicy() {
		t.Error("assembled Services must share the base's ACL policy")
	}

	// Per-store, genuinely independent.
	if a.Store() == b.Store() {
		t.Fatal("two assemblies must not share one store")
	}
	ctx := context.Background()
	if err := a.Store().CreateEntity(ctx, entity.New("DOC-1", "doc")); err != nil {
		t.Fatalf("write to store A: %v", err)
	}
	if _, err := b.Store().GetEntity(ctx, "DOC-1"); err == nil {
		t.Fatal("store B saw store A's write — the stores are not isolated")
	}
}

// TestSharedBase_CloseIsPerAssembly pins the eviction property a multi-store
// host depends on: closing one assembled Services must not disturb the base or
// any sibling. Services.Close tears down only the store and search closer it
// was assembled with; if it ever grew to close something shared, evicting one
// tenant would break every other.
func TestSharedBase_CloseIsPerAssembly(t *testing.T) {
	base := newSharedBase(t)

	a := assembleOver(t, base)
	b := assembleOver(t, base)
	t.Cleanup(func() { _ = b.Close() })

	// Capture the shared face before the close so we can prove the close did
	// not reach through it. Checking only the sibling is not enough: a Close that
	// nils its OWN reference to shared state still signals that teardown is
	// touching things it does not own.
	sharedMeta := base.Meta()

	if err := a.Close(); err != nil {
		t.Fatalf("Close A: %v", err)
	}

	// The base is untouched.
	if base.Meta() != sharedMeta {
		t.Error("closing an assembly mutated the base's metamodel reference")
	}
	// The closed bundle must not have dropped its shared references either —
	// Close owns the store and search closer, nothing else.
	if a.Meta() != sharedMeta {
		t.Error("Close() cleared shared state on the closed Services; it must only " +
			"tear down the store and search closer it was assembled with")
	}

	// B still works.
	ctx := context.Background()
	if err := b.Store().CreateEntity(ctx, entity.New("DOC-2", "doc")); err != nil {
		t.Fatalf("store B unusable after closing A: %v", err)
	}
	if b.Meta() != sharedMeta {
		t.Error("closing one assembly damaged a sibling's metamodel")
	}

	// And the base can still produce new assemblies.
	c := assembleOver(t, base)
	t.Cleanup(func() { _ = c.Close() })
	if c.Meta() != base.Meta() {
		t.Error("base unusable for further assembly after one Services was closed")
	}
}

// TestSharedBase_AssemblyDoesNotMutateSharedValues guards the invariant that
// makes reuse safe at all.
//
// meta and aclPolicy are pointers handed to EVERY assembled Services. If
// assembly wrote through either — registering something on the metamodel, say —
// the effect would leak into every other tenant sharing the base. That is a
// cross-tenant defect with no compile-time signal, so it is pinned here rather
// than left to review.
//
// Comparing entity-type counts and the policy face catches the realistic
// shapes (a mutating consumer appending to a slice/map on the shared value)
// without asserting deep equality on a large struct.
func TestSharedBase_AssemblyDoesNotMutateSharedValues(t *testing.T) {
	base := newSharedBase(t)

	meta := base.Meta()
	typesBefore := len(meta.Entities)
	relsBefore := len(meta.Relations)
	policyBefore := base.Paths()

	for range 3 {
		svc := assembleOver(t, base)
		t.Cleanup(func() { _ = svc.Close() })
	}

	if got := len(meta.Entities); got != typesBefore {
		t.Errorf("assembly mutated the shared metamodel: entity types %d -> %d", typesBefore, got)
	}
	if got := len(meta.Relations); got != relsBefore {
		t.Errorf("assembly mutated the shared metamodel: relations %d -> %d", relsBefore, got)
	}
	if base.Paths() != policyBefore {
		t.Error("assembly replaced the base's project context")
	}
	if base.Meta() != meta {
		t.Error("assembly replaced the base's metamodel")
	}
}

// TestNewSharedBase_ValidatesUpFront keeps the constructor honest: a base with
// a missing collaborator must fail where it is built, not at first use inside
// an assembly (CLAUDE.md — constructors reject nil required fields).
func TestNewSharedBase_ValidatesUpFront(t *testing.T) {
	root := writeMinimalProject(t)
	fs := storage.NewSafeFS(storage.NewOsFS())
	paths, err := project.Discover(root, fs)
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if _, err := appbuild.NewSharedBase(appbuild.Config{
		FS:    fs,
		Paths: paths,
		// ScriptEngine and Audit deliberately omitted.
	}); err == nil {
		t.Fatal("NewSharedBase must reject a Config missing required collaborators")
	}
}

// TestNewSharedBase_CompilesWorlds pins that world compilation runs at
// assembly (TKT-WAV8XP). The face GRAMMAR is checked in internal/worlds
// rather than the loader — metamodel may not import entity under arch-lint —
// so the boot is the only place left that can turn a bad face name into a
// startup failure instead of a lurking runtime one. Without this call site
// the grammar half of the feature would be enforced nowhere.
func TestNewSharedBase_CompilesWorlds(t *testing.T) {
	newBaseOver := func(t *testing.T, schema string) (*appbuild.SharedBase, error) {
		t.Helper()
		root := t.TempDir()
		if err := os.WriteFile(
			filepath.Join(root, "metamodel.yaml"), []byte(schema), 0o644,
		); err != nil {
			t.Fatal(err)
		}
		for _, dir := range []string{".rela", "entities", "relations"} {
			if err := os.MkdirAll(filepath.Join(root, dir), 0o755); err != nil {
				t.Fatal(err)
			}
		}
		fs := storage.NewSafeFS(storage.NewOsFS())
		paths, err := project.Discover(root, fs)
		if err != nil {
			t.Fatalf("Discover: %v", err)
		}
		return appbuild.NewSharedBase(appbuild.Config{
			FS:           fs,
			Paths:        paths,
			ScriptEngine: script.NewEngine(),
			Audit:        audit.Nop{},
		})
	}

	const good = `version: "1.0"
entities:
  doc:
    label: Doc
    plural: docs
    id_prefix: "DOC-"
    id_type: sequential
    properties:
      title: {type: string}
    bare_face: draft
    faces:
      draft: {}
      published: {}
worlds:
  published:
    select: published
    otherwise: exclude
`

	t.Run("a declared world is compiled and reachable", func(t *testing.T) {
		base, err := newBaseOver(t, good)
		if err != nil {
			t.Fatalf("NewSharedBase: %v", err)
		}
		if _, ok := base.Worlds().Lookup("published"); !ok {
			t.Error("the declared world must be compiled onto the base")
		}
	})

	t.Run("an invalid face name fails the boot", func(t *testing.T) {
		// `Draft` is not a legal face name (no uppercase). The loader's
		// structural checks pass it; only the compiler catches it.
		bad := strings.Replace(good, "draft: {}", "Draft: {}", 1)
		bad = strings.Replace(bad, "bare_face: draft", "bare_face: Draft", 1)
		_, err := newBaseOver(t, bad)
		if err == nil {
			t.Fatal("NewSharedBase must reject an invalid face name at startup")
		}
		for _, want := range []string{"doc", "Draft"} {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("error must name %q, got: %v", want, err)
			}
		}
	})
}
