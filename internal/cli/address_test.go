package cli

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/output"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
	"github.com/Sourcehaven-BV/rela/internal/worlds"
)

// addressFixture stores a faceless requirement REQ-1 and a faced page PG-1
// with only its draft and published faces, as a faced type is stored
// (DEC-NPZICR).
func addressFixture(t *testing.T) *memstore.MemStore {
	t.Helper()
	st := memstore.New()
	ctx := context.Background()
	for _, e := range []*entity.Entity{
		{ID: "REQ-1", Type: "requirement", Properties: map[string]any{"title": "req"}},
		{ID: "PG-1", Type: "page", Face: "draft", Properties: map[string]any{"title": "draft page"}},
		{ID: "PG-1", Type: "page", Face: "published", Properties: map[string]any{"title": "live page"}},
	} {
		if err := st.CreateEntity(ctx, e); err != nil {
			t.Fatalf("seed %s: %v", e.ID, err)
		}
	}
	return st
}

func TestReadAddress(t *testing.T) {
	t.Parallel()
	st := addressFixture(t)
	world := store.TrivialScope() // the default world

	tests := []struct {
		name      string
		addr      string
		wantTitle string
		wantErr   error
		wantMsg   string
	}{
		{name: "faceless bare id", addr: "REQ-1", wantTitle: "req"},
		{name: "faced ID@face", addr: "PG-1@draft", wantTitle: "draft page"},
		{name: "faced other face", addr: "PG-1@published", wantTitle: "live page"},
		{
			name: "faced bare id names its faces", addr: "PG-1", wantErr: errFaceRequired,
			wantMsg: "PG-1@draft, PG-1@published",
		},
		{name: "missing face", addr: "PG-1@review", wantErr: store.ErrNotFound},
		{name: "missing entity", addr: "NOPE-1", wantErr: store.ErrNotFound},
		{name: "face on a faceless entity", addr: "REQ-1@draft", wantErr: store.ErrNotFound},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := readAddress(context.Background(), st, store.TrivialScope(), world, tc.addr)
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("readAddress(%q) err = %v, want %v", tc.addr, err, tc.wantErr)
				}
				if tc.wantMsg != "" && !strings.Contains(err.Error(), tc.wantMsg) {
					t.Errorf("error %q does not name %q", err, tc.wantMsg)
				}
				return
			}
			if err != nil {
				t.Fatalf("readAddress(%q): %v", tc.addr, err)
			}
			if got.Title() != tc.wantTitle {
				t.Errorf("readAddress(%q) title = %q, want %q", tc.addr, got.Title(), tc.wantTitle)
			}
		})
	}
}

// TestReadAddress_FacesMessageUsesDeclarationOrder pins that the faces a bare
// id lists come in the schema's declaration order, not by name (RR-V3UH7K).
func TestReadAddress_FacesMessageUsesDeclarationOrder(t *testing.T) {
	t.Parallel()
	meta, err := metamodel.Parse([]byte(`
entities:
  requirement:
    label: Requirement
    id_prefix: REQ
    properties:
      title: {type: string}
  page:
    label: Page
    id_prefix: PG
    faces:
      published: {}
      draft: {}
    properties:
      title: {type: string}
`))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	compiled, err := worlds.Compile(meta)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	// A world that serves no face of a page, so the bare id lists its faces.
	_, err = readAddress(context.Background(), addressFixture(t), compiled.Families(), store.TrivialScope(), "PG-1")
	if !errors.Is(err, errFaceRequired) {
		t.Fatalf("err = %v, want %v", err, errFaceRequired)
	}
	if want := "PG-1@published, PG-1@draft"; !strings.Contains(err.Error(), want) {
		t.Errorf("error %q does not list %q", err, want)
	}

	// The generated default world serves the first declared face the
	// entity has (TKT-7IZHP0 §3.2), so the same bare id now resolves.
	got, err := readAddress(context.Background(), addressFixture(t), compiled.Families(),
		compiled.DefaultWorld(), "PG-1")
	if err != nil {
		t.Fatalf("readAddress(PG-1) in the generated world: %v", err)
	}
	if got.Title() != "live page" {
		t.Errorf("title = %q, want the published face", got.Title())
	}
}

func TestRowsInWorld(t *testing.T) {
	t.Parallel()
	st := addressFixture(t)
	world := store.TrivialScope()

	got := rowsInWorld(context.Background(), st, world, []string{"REQ-1", "PG-1", "NOPE-1"})
	if e := got["REQ-1"]; e == nil || e.Title() != "req" {
		t.Errorf("REQ-1 = %+v, want the faceless row", e)
	}
	// The trivial scope resolves no face of a faced type,
	// so the caller shows its id rather than an arbitrary face's title.
	if e, ok := got["PG-1"]; ok {
		t.Errorf("PG-1 = %+v, want absent in the default world", e)
	}
	if len(rowsInWorld(context.Background(), st, world, nil)) != 0 {
		t.Error("no ids must load nothing")
	}
}

// TestReadAddress_WorldSelectsAFace pins that a bare id reads the face its
// world selects, so a world that resolves faced types (TKT-7IZHP0) needs no
// change in the CLI address helpers.
func TestReadAddress_WorldSelectsAFace(t *testing.T) {
	t.Parallel()
	st := addressFixture(t)
	world := store.NewWorldScope(map[string]store.TypeResolution{"page": {Chain: []entity.Face{"draft"}}})

	got, err := readAddress(context.Background(), st, store.TrivialScope(), world, "PG-1")
	if err != nil {
		t.Fatalf("readAddress(PG-1): %v", err)
	}
	if got.Title() != "draft page" {
		t.Errorf("readAddress(PG-1) title = %q, want the draft face", got.Title())
	}
	if e := rowsInWorld(context.Background(), st, world, []string{"PG-1"})["PG-1"]; e == nil || e.Title() != "draft page" {
		t.Errorf("rowsInWorld PG-1 = %+v, want the draft face", e)
	}
}

func TestRequireAddressExists(t *testing.T) {
	t.Parallel()
	st := addressFixture(t)
	tests := []struct {
		addr string
		ok   bool
	}{
		{"REQ-1", true},
		{"PG-1", true}, // the entity exists, though it has no zero face
		{"PG-1@draft", true},
		{"PG-1@review", false},
		{"NOPE-1", false},
		{"@@bad", false},
	}
	for _, tc := range tests {
		t.Run(tc.addr, func(t *testing.T) {
			t.Parallel()
			err := requireAddressExists(context.Background(), st, tc.addr)
			if (err == nil) != tc.ok {
				t.Errorf("requireAddressExists(%q) = %v, want ok=%v", tc.addr, err, tc.ok)
			}
		})
	}
}

// TestShowFacedBareID pins the CLI side of readAddress: `rela show` of a bare
// id of a faced entity refuses and names its faces, where it used to report
// the entity as not found.
func TestShowFacedBareID(t *testing.T) {
	st := addressFixture(t)
	svc := &readServices{Store: st, World: store.TrivialScope()}
	withOutput(t, output.FormatTable)

	err := (&ShowCmd{ID: "PG-1"}).Run(context.Background(), svc)
	if err == nil || !strings.Contains(err.Error(), "PG-1@draft") {
		t.Fatalf("show PG-1 = %v, want an error naming PG-1@draft", err)
	}
}
