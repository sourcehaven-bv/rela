package dataentry

import (
	"context"
	"errors"
	"iter"
	"sync"
	"testing"

	v1 "github.com/Sourcehaven-BV/rela/internal/apiwire/v1"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
)

func TestCollectMentions(t *testing.T) {
	t.Parallel()

	seed := []*entity.Entity{
		mustEntity("TKT-LXYHQ", "ticket", "Resolve entity-ID references"),
		mustEntity("FEAT-010", "feature", "Use goldmark for markdown rendering"),
		mustEntityNamed("data-entry-ui", "concept", "Data Entry Web UI"),
		// Locked entity whose content (and therefore display title) is
		// unreadable — link must render with the inaccessible affordance.
		lockedEntity("TKT-LOCKED", "ticket", entity.InaccessibleReasonGitCrypt),
		// Locked-but-readable: a property OTHER than the display source
		// (`status`) is locked. The display title is still readable, so
		// the resulting mention must NOT carry the inaccessible flag.
		entityWithLockedProperty("TKT-PARTIAL", "ticket", "Mostly Readable", "status"),
	}

	tests := []struct {
		name     string
		contents []string
		want     map[string]v1.Mention
	}{
		{
			name:     "known short-ID code span resolves",
			contents: []string{"see `TKT-LXYHQ` for context"},
			want: map[string]v1.Mention{
				"TKT-LXYHQ": {Type: "ticket", Title: "Resolve entity-ID references"},
			},
		},
		{
			name:     "manual-ID concept resolves via DisplayTitle (name)",
			contents: []string{"covered by `data-entry-ui`"},
			want: map[string]v1.Mention{
				// concept's primary property is `name` per its metamodel;
				// the title comes from there, not the `title` property
				// (which the seed never sets for concept).
				"data-entry-ui": {Type: "concept", Title: "Data Entry Web UI"},
			},
		},
		{
			name:     "unknown ID is dropped",
			contents: []string{"`TKT-NOPE` is not real"},
			want:     nil,
		},
		{
			name:     "multi-token code span is not collected",
			contents: []string{"`TKT-LXYHQ and FEAT-010` mention both"},
			want:     nil,
		},
		{
			name:     "ID inside fenced code block is ignored",
			contents: []string{"prose\n\n```\nTKT-LXYHQ in a block\n```\n"},
			want:     nil,
		},
		{
			name:     "ID inside indented code block is ignored",
			contents: []string{"prose\n\n    TKT-LXYHQ indented\n"},
			want:     nil,
		},
		{
			name:     "link text containing an ID is not a code span",
			contents: []string{"see [TKT-LXYHQ](https://example.com)"},
			want:     nil,
		},
		{
			name:     "multiple distinct mentions are deduplicated across blobs",
			contents: []string{"`TKT-LXYHQ` and `FEAT-010`", "again: `TKT-LXYHQ`"},
			want: map[string]v1.Mention{
				"TKT-LXYHQ": {Type: "ticket", Title: "Resolve entity-ID references"},
				"FEAT-010":  {Type: "feature", Title: "Use goldmark for markdown rendering"},
			},
		},
		{
			name:     "inaccessible target carries inaccessible + reason",
			contents: []string{"locked: `TKT-LOCKED`"},
			want: map[string]v1.Mention{
				"TKT-LOCKED": {
					Type:               "ticket",
					Title:              "TKT-LOCKED",
					Inaccessible:       true,
					InaccessibleReason: "git-crypt",
				},
			},
		},
		{
			name:     "partially-locked entity keeps its readable title and is NOT inaccessible",
			contents: []string{"partial: `TKT-PARTIAL`"},
			want: map[string]v1.Mention{
				"TKT-PARTIAL": {Type: "ticket", Title: "Mostly Readable"},
			},
		},
		{
			name:     "code span inside list item is collected",
			contents: []string{"- see `TKT-LXYHQ` here\n"},
			want: map[string]v1.Mention{
				"TKT-LXYHQ": {Type: "ticket", Title: "Resolve entity-ID references"},
			},
		},
		{
			name:     "code span inside blockquote is collected",
			contents: []string{"> quoted: `TKT-LXYHQ`\n"},
			want: map[string]v1.Mention{
				"TKT-LXYHQ": {Type: "ticket", Title: "Resolve entity-ID references"},
			},
		},
		{
			name: "code span inside GFM table cell is collected",
			contents: []string{
				"| col |\n| --- |\n| `TKT-LXYHQ` |\n",
			},
			want: map[string]v1.Mention{
				"TKT-LXYHQ": {Type: "ticket", Title: "Resolve entity-ID references"},
			},
		},
		{
			name:     "empty content returns nil",
			contents: []string{""},
			want:     nil,
		},
	}

	meta := buildTestMetamodel(t)
	st := memstore.New()
	for _, e := range seed {
		if err := st.CreateEntity(context.Background(), e); err != nil {
			t.Fatalf("seed %q: %v", e.ID, err)
		}
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := collectMentions(context.Background(), st, meta, tc.contents...)
			assertMentionsEqual(t, tc.want, got)
		})
	}
}

func TestCollectMentions_SelfReference(t *testing.T) {
	t.Parallel()

	self := mustEntity("TKT-SELF", "ticket", "Looking inward")
	meta := buildTestMetamodel(t)
	st := memstore.New()
	if err := st.CreateEntity(context.Background(), self); err != nil {
		t.Fatalf("seed self: %v", err)
	}

	got := collectMentions(context.Background(), st, meta, "see `TKT-SELF` for the recursion")
	want := map[string]v1.Mention{
		"TKT-SELF": {Type: self.Type, Title: self.Title()},
	}
	assertMentionsEqual(t, want, got)
}

func TestCollectMentions_ContextCancellationStops(t *testing.T) {
	t.Parallel()

	// Cancelled context — the per-ID loop must bail out before touching the
	// store. Even with a known candidate it returns nil because we never
	// resolved anything.
	meta := buildTestMetamodel(t)
	st := memstore.New()
	if err := st.CreateEntity(context.Background(), mustEntity("TKT-CTX", "ticket", "T")); err != nil {
		t.Fatalf("seed: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	got := collectMentions(ctx, st, meta, "`TKT-CTX`")
	if got != nil {
		t.Errorf("expected nil on cancelled context, got %+v", got)
	}
}

func TestCollectMentions_StoreErrorIsLoggedAndSkipped(t *testing.T) {
	t.Parallel()

	// A flaky store error must not break the whole view-fetch response —
	// it degrades to "code span stays as <code>" (the same UX as
	// unknown-ID), and the bad ID drops out of the result.
	meta := buildTestMetamodel(t)
	flaky := &flakyStore{
		err:  errors.New("backend offline"),
		good: mustEntity("TKT-OK", "ticket", "Resolves fine"),
	}

	got := collectMentions(context.Background(), flaky, meta, "`TKT-FAIL` then `TKT-OK`")
	want := map[string]v1.Mention{
		"TKT-OK": {Type: "ticket", Title: "Resolves fine"},
	}
	assertMentionsEqual(t, want, got)
}

func TestCollectMentions_ConcurrentScanIsSafe(t *testing.T) {
	t.Parallel()

	// `mentionsMarkdown` is a package-level goldmark.Markdown shared by all
	// callers. Verify goroutine safety: many concurrent scans of the same
	// content produce the same result every time.
	meta := buildTestMetamodel(t)
	st := memstore.New()
	if err := st.CreateEntity(context.Background(), mustEntity("TKT-CONC", "ticket", "Concurrent")); err != nil {
		t.Fatalf("seed: %v", err)
	}

	const n = 64
	var wg sync.WaitGroup
	wg.Add(n)
	results := make([]map[string]v1.Mention, n)
	for i := range n {
		go func(idx int) {
			defer wg.Done()
			results[idx] = collectMentions(context.Background(), st, meta, "ref to `TKT-CONC`")
		}(i)
	}
	wg.Wait()

	want := map[string]v1.Mention{"TKT-CONC": {Type: "ticket", Title: "Concurrent"}}
	for i, got := range results {
		if len(got) != 1 || got["TKT-CONC"] != want["TKT-CONC"] {
			t.Fatalf("goroutine %d: want %+v, got %+v", i, want, got)
		}
	}
}

// buildTestMetamodel returns a small metamodel covering the entity types
// the table-driven tests above seed: `ticket` (primary=title) and
// `concept` (primary=name) — the latter is what exercises the
// DisplayTitle path for `id_type: manual` entities like data-entry-ui.
func buildTestMetamodel(t *testing.T) *metamodel.Metamodel {
	t.Helper()
	// GetPrimaryProperty picks `title`/`name` automatically when defined
	// as a required string. Status property is included to exercise the
	// "locked non-display property" case.
	return &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"ticket": {
				Label: "Ticket",
				Properties: map[string]metamodel.PropertyDef{
					"title":  {Type: "string", Required: true},
					"status": {Type: "string"},
				},
			},
			"feature": {
				Label: "Feature",
				Properties: map[string]metamodel.PropertyDef{
					"title": {Type: "string", Required: true},
				},
			},
			"concept": {
				Label: "Concept",
				Properties: map[string]metamodel.PropertyDef{
					"name": {Type: "string", Required: true},
				},
			},
		},
	}
}

func mustEntity(id, typeName, title string) *entity.Entity {
	e := entity.New(id, typeName)
	e.Properties["title"] = title
	return e
}

// mustEntityNamed builds an entity whose display title comes from `name`
// (the primary property for `concept`) rather than `title`. Used to
// exercise the DisplayTitle path with `id_type: manual` types.
func mustEntityNamed(id, typeName, name string) *entity.Entity {
	e := entity.New(id, typeName)
	e.Properties["name"] = name
	return e
}

func lockedEntity(id, typeName string, reason entity.InaccessibleReason) *entity.Entity {
	e := entity.New(id, typeName)
	// Lock the entity by marking content inaccessible — matches what the
	// markdown loader produces for git-crypt encrypted files. The display
	// title is therefore unknown, so the SPA renders the link with a lock.
	e.Inaccessible = []entity.InaccessibleField{
		{Name: entity.InaccessibleFieldContent, Reason: reason},
	}
	return e
}

// entityWithLockedProperty produces an entity whose `title` is readable
// but some unrelated property is locked. The wire-shape v1.Mention should
// keep `Inaccessible == false` because the displayed link text comes
// from a readable source.
func entityWithLockedProperty(id, typeName, title, lockedProp string) *entity.Entity {
	e := entity.New(id, typeName)
	e.Properties["title"] = title
	e.Inaccessible = []entity.InaccessibleField{
		{Name: lockedProp, Reason: entity.InaccessibleReasonGitCrypt},
	}
	return e
}

func assertMentionsEqual(t *testing.T, want, got map[string]v1.Mention) {
	t.Helper()
	if len(want) != len(got) {
		t.Fatalf("mention count mismatch: want %d, got %d (want=%v got=%v)",
			len(want), len(got), want, got)
	}
	for id, w := range want {
		g, ok := got[id]
		if !ok {
			t.Fatalf("missing mention %q (got=%v)", id, got)
		}
		if g != w {
			t.Fatalf("mention %q mismatch: want %+v, got %+v", id, w, g)
		}
	}
}

// flakyStore is an EntityReader test double: GetEntity returns `err` for
// every lookup except the one matching `good.ID`. Other EntityReader
// methods panic — collectMentions does not call them.
type flakyStore struct {
	err  error
	good *entity.Entity
}

func (f *flakyStore) GetEntity(_ context.Context, id string) (*entity.Entity, error) {
	if f.good != nil && f.good.ID == id {
		return f.good, nil
	}
	return nil, f.err
}

func (f *flakyStore) ListEntities(_ context.Context, _ store.EntityQuery) iter.Seq2[*entity.Entity, error] {
	panic("flakyStore: ListEntities not implemented")
}

func (f *flakyStore) ListEntitiesPage(_ context.Context, _ store.EntityQuery) (store.Page[*entity.Entity], error) {
	panic("flakyStore: ListEntitiesPage not implemented")
}

func (f *flakyStore) CountEntities(_ context.Context, _ store.EntityQuery) (int, error) {
	panic("flakyStore: CountEntities not implemented")
}

func (f *flakyStore) HighestID(_ context.Context, _ string) (int, error) {
	panic("flakyStore: HighestID not implemented")
}

func (f *flakyStore) PropertyValues(_ context.Context, _ string, _ int) ([]string, error) {
	panic("flakyStore: PropertyValues not implemented")
}
