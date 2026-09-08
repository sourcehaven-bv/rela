package entitymanager_test

import (
	"context"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/entitymanager"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/statemachine"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
)

// originMetaYAML declares one same-entity copy (page@live -> page@review) and
// one CROSS-entity copy, so the boundary can be checked on both shapes: the
// cross-entity form is the one whose source id is a different row, which is
// what the read-out gate exists for.
const originMetaYAML = `
version: "1"
entities:
  page:
    label: Page
    id_prefix: PAGE
    bare_face: live
    faces:
      live: {}
      review: {}
    properties:
      title: {type: string}
  note:
    label: Note
    id_prefix: NOTE
    properties:
      title: {type: string}
  doc:
    label: Doc
    id_prefix: DOC
    # No bare_face, so face "en" is NON-bare: its stored coordinate and its
    # declared name are the same string. The control for the bare-face case.
    faces:
      en: {}
      nl: {}
    properties:
      title: {type: string}
copies:
  stage-review:
    from: page@live
    to: page@review
    fields: all
    guard:
      permission: stage
  note-from-page:
    from: page@live
    to: note
    fields:
      title: "{{new.title}}"
  translate:
    from: doc@en
    to: doc@nl
    fields:
      title: "{{new.title}}"
    guard:
      permission: stage
  # Declares the source WITHOUT a face suffix, so from.Face is "". It
  # addresses the same face "page@live" does (live is the bare face), so it
  # must label it the same way — the declared name, from the coordinate.
  stage-review-bare:
    from: page
    to: page@review
    fields:
      title: "{{new.title}}"
    guard:
      permission: stage
`

// originRecordingStore wraps a store and records the store.Origin carried on
// the ctx of every entity write, so a test can assert what the WRITE BOUNDARY
// handed the store — which is the only sanctioned route provenance travels.
type originRecordingStore struct {
	store.Store
	created []store.Origin
	updated []store.Origin
}

func (s *originRecordingStore) CreateEntity(ctx context.Context, e *entity.Entity) error {
	s.created = append(s.created, store.OriginFrom(ctx))
	return s.Store.CreateEntity(ctx, e)
}

func (s *originRecordingStore) UpdateEntity(ctx context.Context, e *entity.Entity) error {
	s.updated = append(s.updated, store.OriginFrom(ctx))
	return s.Store.UpdateEntity(ctx, e)
}

// Tx must be re-implemented, not inherited, and the view it hands the callback
// must record into THIS recorder.
//
// A copy does every one of its writes through the transaction VIEW, never
// through the outer store (applyCopy's whole contract). Inheriting memstore's
// Tx would hand the callback memstore's own view, the wrapper would see no
// writes at all, and every assertion below would pass vacuously on an empty
// slice — the test would be measuring nothing.
func (s *originRecordingStore) Tx(ctx context.Context, fn func(store.Store) error) error {
	tr, ok := s.Store.(store.Transactor)
	if !ok {
		return fn(s)
	}
	return tr.Tx(ctx, func(view store.Store) error {
		return fn(&txRecordingStore{Store: view, rec: s})
	})
}

// txRecordingStore records a transaction view's writes back into the outer
// recorder, so assertions can read them after the transaction has committed.
type txRecordingStore struct {
	store.Store
	rec *originRecordingStore
}

func (s *txRecordingStore) CreateEntity(ctx context.Context, e *entity.Entity) error {
	s.rec.created = append(s.rec.created, store.OriginFrom(ctx))
	return s.Store.CreateEntity(ctx, e)
}

func (s *txRecordingStore) UpdateEntity(ctx context.Context, e *entity.Entity) error {
	s.rec.updated = append(s.rec.updated, store.OriginFrom(ctx))
	return s.Store.UpdateEntity(ctx, e)
}

func newOriginRecorder(inner store.Store) *originRecordingStore {
	return &originRecordingStore{Store: inner}
}

// TestCopyStampsOriginOnItsWrites pins the boundary contract: a copy's writes
// carry a store.Origin naming the mechanism, the definition and the SOURCE
// face, while an ordinary write through the same manager carries none.
//
// This is the load-bearing half of "history can tell a copy from a hand edit".
// The store column and the sweep are downstream of it; if the boundary does
// not stamp, everything downstream records a hand edit.
func TestCopyStampsOriginOnItsWrites(t *testing.T) {
	tests := []struct {
		name       string
		definition string
		sourceID   string
		targetID   string
		want       store.Origin
		wantLabel  string
	}{
		{
			name:       "BARE face source records its DECLARED name",
			definition: "stage-review",
			sourceID:   "PAGE-1",
			want: store.Origin{
				Kind: store.OriginCopy,
				// Same id as the target — the FACE is what differs, and the
				// pair is the source. Recording the id anyway keeps the
				// reader from having to know that.
				Source: "PAGE-1",
				// `bare_face: live` makes `live` the ZERO stored coordinate,
				// so recording the coordinate here would record "" and read
				// back as a bare "PAGE-1" — dropping the one fact provenance
				// carries. The boundary records the DECLARED name instead.
				SourceFace: "live",
				SourceType: "page",
				Definition: "stage-review",
			},
			wantLabel: "PAGE-1@live",
		},
		{
			name: "source declared without @face still labels the face it addresses",
			// `from: page` and `from: page@live` address the SAME face, so
			// they must produce the same label; the name is resolved from the
			// coordinate, not read off the definition text.
			definition: "stage-review-bare",
			sourceID:   "PAGE-1",
			want: store.Origin{
				Kind:       store.OriginCopy,
				Source:     "PAGE-1",
				SourceFace: "live",
				SourceType: "page",
				Definition: "stage-review-bare",
			},
			wantLabel: "PAGE-1@live",
		},
		{
			name: "NON-bare face source is unchanged",
			// The control: `en` is not doc's bare face, so its declared name
			// and stored coordinate coincide and nothing about this case
			// moves. If the fix had been "always spell something", this is
			// where a wrong spelling would show up.
			definition: "translate",
			sourceID:   "DOC-1",
			want: store.Origin{
				Kind:       store.OriginCopy,
				Source:     "DOC-1",
				SourceFace: "en",
				SourceType: "doc",
				Definition: "translate",
			},
			wantLabel: "DOC-1@en",
		},
		{
			name:       "cross-entity copy names the source ENTITY",
			definition: "note-from-page",
			sourceID:   "PAGE-1",
			targetID:   "NOTE-1",
			want: store.Origin{
				Kind:       store.OriginCopy,
				Source:     "PAGE-1",
				SourceFace: "live",
				SourceType: "page",
				Definition: "note-from-page",
			},
			wantLabel: "PAGE-1@live",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rec, mgr := newOriginManager(t)
			ctx := principal.With(context.Background(),
				principal.Principal{User: "edith@example.com", Tool: "data-entry"})

			if _, err := mgr.CopyState(ctx, entitymanager.CopyRequest{
				Definition: tc.definition, SourceID: tc.sourceID, TargetID: tc.targetID,
			}); err != nil {
				t.Fatalf("copy: %v", err)
			}

			got := append(append([]store.Origin{}, rec.created...), rec.updated...)
			if len(got) == 0 {
				t.Fatal("copy performed no recorded entity write")
			}
			for i, o := range got {
				if o != tc.want {
					t.Errorf("write %d origin = %+v, want %+v", i, o, tc.want)
				}
				// The label is what a reader actually sees, and it is the
				// single spelling the CLI and the SPA share, so assert it
				// here rather than trusting the field alone.
				if got := o.SourceLabel(); got != tc.wantLabel {
					t.Errorf("write %d SourceLabel() = %q, want %q", i, got, tc.wantLabel)
				}
			}
		})
	}
}

// TestOrdinaryWriteCarriesNoOrigin is the other half of the user's ask:
// "manual edits should be marked similarly". The marking is the ABSENCE of an
// origin — there is deliberately no OriginKind("manual") — so this pins that a
// plain manager write leaves the ctx unmarked and the row records NULL.
//
// Without this test the copy assertion above would still pass if the boundary
// stamped every write, which would make the marker meaningless.
func TestOrdinaryWriteCarriesNoOrigin(t *testing.T) {
	rec, mgr := newOriginManager(t)
	ctx := principal.With(context.Background(),
		principal.Principal{User: "edith@example.com", Tool: "data-entry"})

	if _, err := mgr.PatchEntity(ctx, "PAGE-1", entity.Patch{
		Properties: map[string]any{"title": "typed by hand"},
	}); err != nil {
		t.Fatalf("patch: %v", err)
	}

	all := append(append([]store.Origin{}, rec.created...), rec.updated...)
	if len(all) == 0 {
		t.Fatal("patch performed no recorded entity write")
	}
	for i, o := range all {
		if !o.IsZero() {
			t.Errorf("write %d carried origin %+v; a hand edit must carry NONE "+
				"(absence is the signal, and a marked hand edit would make "+
				"the copy marker meaningless)", i, o)
		}
	}
}

// newOriginManager builds a manager over an origin-recording memstore seeded
// with PAGE-1's default face.
func newOriginManager(t *testing.T) (*originRecordingStore, *entitymanager.Manager) {
	t.Helper()
	rec := newOriginRecorder(memstore.New())
	meta, err := metamodel.Parse([]byte(originMetaYAML))
	if err != nil {
		t.Fatalf("metamodel.Parse: %v", err)
	}
	mgr, err := entitymanager.New(entitymanager.Deps{
		Store:       rec,
		Meta:        meta,
		Templater:   nopTemplater{},
		Audit:       audit.Nop{},
		ACL:         acl.NopACL{},
		Transitions: statemachine.EmptySet(),
		FieldGate:   entitymanager.AllowAllFieldGate{},
		CopyGuard:   allowGuard{allow: true},
	})
	if err != nil {
		t.Fatalf("entitymanager.New: %v", err)
	}
	if cerr := rec.Store.CreateEntity(context.Background(), &entity.Entity{
		ID: "PAGE-1", Type: "page", Properties: map[string]any{"title": "live"},
	}); cerr != nil {
		t.Fatalf("seed: %v", cerr)
	}
	// DOC-1@en: doc declares no bare_face, so "en" is a real, non-zero stored
	// coordinate and the source must be seeded AT it. The default row comes
	// first because a face row cannot exist headless.
	for _, e := range []*entity.Entity{
		{ID: "DOC-1", Type: "doc", Properties: map[string]any{"title": "doc"}},
		{ID: "DOC-1", Type: "doc", Face: "en", Properties: map[string]any{"title": "english"}},
	} {
		if cerr := rec.Store.CreateEntity(context.Background(), e); cerr != nil {
			t.Fatalf("seed doc %q: %v", e.Face, cerr)
		}
	}
	return rec, mgr
}
