package analysis_test

import (
	"context"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/analysis"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/lua"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/storage"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/fsstore"
	"github.com/Sourcehaven-BV/rela/internal/tracer"
)

func mustFace(t *testing.T, v string) entity.Face {
	t.Helper()
	p, err := entity.ParseFace(v)
	if err != nil {
		t.Fatalf("ParseFace(%q): %v", v, err)
	}
	return p
}

// TestCheckStates pins the content-state integrity findings
// (TKT-DOFYR1): undeclared faces (count + example refs per face —
// in Step 1 NO face is declarable, so every state reports),
// headless families, and type-mismatched states. The latter two shapes
// are write-path-rejected, so the test seeds them through memstore
// internals-equivalent means: a valid family for the face finding
// and direct store writes for the tolerated-on-disk shapes.
func TestCheckStates(t *testing.T) {
	meta := &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"page": {Label: "Page", IDPrefixes: []string{"PAGE-"}},
		},
	}

	svc := newServiceWith(t, meta, func(s store.Store) {
		addEntity(s, "PAGE-1", "page", map[string]any{"title": "default"})
		draft := entity.New("PAGE-1", "page")
		draft.Face = mustFace(t, "draft")
		if err := s.CreateEntity(context.Background(), draft); err != nil {
			t.Fatal(err)
		}
		review := entity.New("PAGE-1", "page")
		review.Face = mustFace(t, "review")
		if err := s.CreateEntity(context.Background(), review); err != nil {
			t.Fatal(err)
		}
		addEntity(s, "PAGE-2", "page", map[string]any{"title": "plain"})
	})

	findings, err := svc.CheckStates(context.Background(), analysis.Options{})
	if err != nil {
		t.Fatalf("CheckStates: %v", err)
	}

	// Two undeclared faces, sorted, each with count + example refs.
	if len(findings) != 2 {
		t.Fatalf("got %d findings, want 2: %+v", len(findings), findings)
	}
	first := findings[0]
	if first.Code != "undeclared-face" || first.Subject != "draft" ||
		first.Count != 1 || first.Examples[0] != "PAGE-1@draft" {

		t.Errorf("finding[0] = %+v", first)
	}
	if findings[1].Subject != "review" {
		t.Errorf("finding[1] = %+v", findings[1])
	}

	t.Run("scope filters on the bare id", func(t *testing.T) {
		scoped, err := svc.CheckStates(context.Background(), analysis.Options{
			Scope: map[string]bool{"PAGE-2": true},
		})
		if err != nil {
			t.Fatalf("CheckStates: %v", err)
		}
		if len(scoped) != 0 {
			t.Errorf("got %d findings for a stateless scope, want 0: %+v", len(scoped), scoped)
		}
	})

	t.Run("faceless project is silent", func(t *testing.T) {
		plain := newServiceWith(t, meta, func(s store.Store) {
			addEntity(s, "PAGE-9", "page", nil)
		})
		findings, err := plain.CheckStates(context.Background(), analysis.Options{})
		if err != nil {
			t.Fatalf("CheckStates: %v", err)
		}
		if len(findings) != 0 {
			t.Errorf("got %d findings, want 0: %+v", len(findings), findings)
		}
	})
}

// TestCheckStates_ToleratedDiskShapes covers the two findings the write
// path rejects but the fs load path tolerates (design doc §6): a
// headless family and a type-mismatched state, seeded as hand-written
// files.
func TestCheckStates_ToleratedDiskShapes(t *testing.T) {
	fs := storage.NewMemFS()
	write := func(path, content string) {
		t.Helper()
		if err := fs.MkdirAll("/entities/pages", 0o755); err != nil {
			t.Fatal(err)
		}
		if err := fs.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	// Headless: a state with no default row.
	write("/entities/pages/PAGE-7@draft.md", "---\nid: PAGE-7\ntype: page\n---\n")
	// Type mismatch: default is page, state claims ticket. The state
	// file lives in the pages dir (the dir maps the type at scan time),
	// so the mismatch is seeded via a second type's dir.
	write("/entities/pages/PAGE-8.md", "---\nid: PAGE-8\ntype: page\n---\n")
	if err := fs.MkdirAll("/entities/tickets", 0o755); err != nil {
		t.Fatal(err)
	}
	if err := fs.WriteFile("/entities/tickets/PAGE-8@review.md",
		[]byte("---\nid: PAGE-8\ntype: ticket\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	rooted, err := storage.NewRootedFS(fs, "/")
	if err != nil {
		t.Fatal(err)
	}
	st, err := fsstore.New(fsstore.Config{
		FS: fs, Rooted: rooted,
		EntitiesKey: "entities", RelationsKey: "relations",
		AttachmentsKey: "attachments", CacheKey: ".rela",
		Schemas: map[string]store.EntityTypeSchema{
			"page":   {Plural: "pages"},
			"ticket": {Plural: "tickets"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })

	meta := &metamodel.Metamodel{Entities: map[string]metamodel.EntityDef{
		"page":   {Label: "Page"},
		"ticket": {Label: "Ticket"},
	}}
	tr := tracer.New(st)
	svc, err := analysis.New(analysis.Deps{Store: st, Meta: meta, Tracer: tr,
		LuaReadDeps: lua.ReadDeps{VisibleReader: st, Tracer: tr, Meta: meta}})
	if err != nil {
		t.Fatal(err)
	}

	findings, err := svc.CheckStates(context.Background(), analysis.Options{})
	if err != nil {
		t.Fatalf("CheckStates: %v", err)
	}

	byCode := map[string][]analysis.StateFinding{}
	for _, f := range findings {
		byCode[f.Code] = append(byCode[f.Code], f)
	}
	if len(byCode["undeclared-face"]) != 2 {
		t.Errorf("undeclared-face findings = %+v, want draft + review", byCode["undeclared-face"])
	}
	if hf := byCode["headless-family"]; len(hf) != 1 || hf[0].Subject != "PAGE-7" {
		t.Errorf("headless-family findings = %+v, want PAGE-7", hf)
	}
	if tm := byCode["state-type-mismatch"]; len(tm) != 1 || tm[0].Subject != "PAGE-8" {
		t.Errorf("state-type-mismatch findings = %+v, want PAGE-8", tm)
	}
}

// TestCheckStates_SubtractsDeclaredFacesPerType pins AC5 (TKT-WAV8XP):
// the declared set is consulted PER ENTITY TYPE, never flattened.
//
// The negative case is the load-bearing one. A face declared by type A
// but stored on type B must STILL report for B: a type declaring no
// faces contributes its default state to every world, so such a row is
// reachable through no world at all — exactly the stranded data this
// finding exists to surface. Flattening the sets would hide it.
func TestCheckStates_SubtractsDeclaredFacesPerType(t *testing.T) {
	meta := &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"page": {
				Label:      "Page",
				IDPrefixes: []string{"PAGE-"},
				BareFace:   "draft",
				Faces: map[string]metamodel.FaceDef{
					"draft":     {},
					"published": {},
				},
			},
			// policy declares NO faces on purpose.
			"policy": {Label: "Policy", IDPrefixes: []string{"POL-"}},
		},
	}

	seedState := func(s store.Store, id, typ, ptr string) {
		e := entity.New(id, typ)
		e.Face = entity.Face(ptr)
		if err := s.CreateEntity(context.Background(), e); err != nil {
			t.Fatalf("seed %s@%s: %v", id, ptr, err)
		}
	}

	svc := newServiceWith(t, meta, func(s store.Store) {
		addEntity(s, "PAGE-1", "page", map[string]any{"title": "default"})
		seedState(s, "PAGE-1", "page", "published") // declared for page
		seedState(s, "PAGE-1", "page", "legacy")    // declared by nobody

		addEntity(s, "POL-1", "policy", map[string]any{"title": "default"})
		// `draft` IS declared — but by `page`, not by `policy`.
		seedState(s, "POL-1", "policy", "draft")
	})

	findings, err := svc.CheckStates(context.Background(), analysis.Options{})
	if err != nil {
		t.Fatalf("CheckStates: %v", err)
	}

	undeclared := map[string]int{}
	for _, f := range findings {
		if f.Code == "undeclared-face" {
			undeclared[f.Subject] = f.Count
		}
	}

	if _, reported := undeclared["published"]; reported {
		t.Error("`published` is declared for page and must NOT report")
	}
	if got := undeclared["legacy"]; got != 1 {
		t.Errorf("`legacy` is declared by nobody: count = %d, want 1", got)
	}
	if got := undeclared["draft"]; got != 1 {
		t.Errorf("`draft` is declared by page but STORED ON policy, so it must "+
			"report for policy: count = %d, want 1", got)
	}
}

// TestCheckStates_SubtractionResolvesAliases pins that the declared-set
// subtraction resolves entity-type ALIASES.
//
// The write path does not canonicalize an entity's type, so a stored row
// legitimately carries an alias. Indexing Meta.Entities directly (rather
// than going through GetEntityDef, as the rest of the codebase does)
// makes every state of an alias-typed entity look like stranded data —
// a false undeclared-face finding that an operator cannot act on,
// because the face IS declared.
func TestCheckStates_SubtractionResolvesAliases(t *testing.T) {
	meta := &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"page": {
				Label:      "Page",
				IDPrefixes: []string{"PAGE-"},
				Aliases:    []string{"webpage"},
				BareFace:   "draft",
				Faces: map[string]metamodel.FaceDef{
					"draft":     {},
					"published": {},
				},
			},
		},
	}
	// Alias resolution runs off an unexported map that Parse builds; a
	// hand-built Metamodel must ask for it explicitly.
	meta.InitAliases()

	svc := newServiceWith(t, meta, func(s store.Store) {
		addEntity(s, "PAGE-1", "webpage", map[string]any{"title": "default"})
		e := entity.New("PAGE-1", "webpage")
		e.Face = entity.Face("published")
		if err := s.CreateEntity(context.Background(), e); err != nil {
			t.Fatalf("seed alias-typed state: %v", err)
		}
	})

	findings, err := svc.CheckStates(context.Background(), analysis.Options{})
	if err != nil {
		t.Fatalf("CheckStates: %v", err)
	}

	for _, f := range findings {
		if f.Code == "undeclared-face" && f.Subject == "published" {
			t.Errorf("`published` is declared by page, and `webpage` is an alias of page, "+
				"so an alias-typed row must NOT report: count = %d", f.Count)
		}
	}
}

// TestCheckStates_FullyDeclaredProjectIsSilent pins that a project whose
// stored states are all declared produces no undeclared-face findings —
// the adopter's steady state.
func TestCheckStates_FullyDeclaredProjectIsSilent(t *testing.T) {
	meta := &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"page": {
				Label:      "Page",
				IDPrefixes: []string{"PAGE-"},
				BareFace:   "draft",
				Faces: map[string]metamodel.FaceDef{
					"draft":     {},
					"published": {},
				},
			},
		},
	}

	svc := newServiceWith(t, meta, func(s store.Store) {
		addEntity(s, "PAGE-1", "page", map[string]any{"title": "default"})
		for _, ptr := range []string{"draft", "published"} {
			e := entity.New("PAGE-1", "page")
			e.Face = entity.Face(ptr)
			if err := s.CreateEntity(context.Background(), e); err != nil {
				t.Fatal(err)
			}
		}
	})

	findings, err := svc.CheckStates(context.Background(), analysis.Options{})
	if err != nil {
		t.Fatalf("CheckStates: %v", err)
	}
	for _, f := range findings {
		if f.Code == "undeclared-face" {
			t.Errorf("unexpected finding for a fully declared project: %+v", f)
		}
	}
}
