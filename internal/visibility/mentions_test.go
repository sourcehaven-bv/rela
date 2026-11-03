package visibility_test

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
	"github.com/Sourcehaven-BV/rela/internal/store/storetest"
	"github.com/Sourcehaven-BV/rela/internal/visibility"
)

// mentionStore seeds rows whose bodies mention REQ-1: TKT-1, TKT-2, and
// POL-1 at its draft face only. Its published face mentions nothing.
func mentionStore(t *testing.T) *storetest.Counting {
	t.Helper()
	st := memstore.New()
	for _, e := range []*entity.Entity{
		{ID: "TKT-1", Type: "ticket", Content: "see `REQ-1`"},
		{ID: "TKT-2", Type: "ticket", Content: "see `REQ-1` and `REQ-2`"},
		{ID: "TKT-3", Type: "ticket", Content: "nothing"},
		{ID: "POL-1", Type: "policy", Face: faceDraft, Content: "see `REQ-1`"},
		{ID: "POL-1", Type: "policy", Face: facePublished, Content: "no mention"},
	} {
		if err := st.CreateEntity(context.Background(), e); err != nil {
			t.Fatalf("seed %s: %v", e.Ref(), err)
		}
	}
	return storetest.NewCounting(st)
}

func pageRefs(p visibility.MentionPage) []string {
	out := make([]string, 0, len(p.Items))
	for _, h := range p.Items {
		out = append(out, entity.FormatStateRef(h.ID, h.Face))
	}
	return out
}

func TestResolver_MentionedBy(t *testing.T) {
	for _, tc := range []struct {
		name  string
		gate  resolverGate
		world visibility.World
		want  []string
	}{
		{
			name:  "default world lists every mentioning row",
			world: visibility.WorldOf(store.TrivialScope()),
			want:  []string{"TKT-1", "TKT-2"},
		},
		{
			name: "a faced source is listed at the face the world serves when that face mentions",
			world: visibility.WorldOf(store.NewWorldScope(map[string]store.TypeResolution{
				"policy": {Chain: []entity.Face{faceDraft, facePublished}, Fallback: store.FallbackExclude},
			})),
			want: []string{"POL-1@draft", "TKT-1", "TKT-2"},
		},
		{
			name:  "a served face that does not mention hides the backlink",
			world: visibility.WorldOf(publishedWorld()),
			want:  []string{"TKT-1", "TKT-2"},
		},
		{
			name:  "a mentioning face the principal cannot read is not served",
			gate:  resolverGate{faces: map[string]visibility.FaceSet{"policy": visibility.SomeFaces(facePublished)}},
			world: visibility.WorldOf(publishedWorld()),
			want:  []string{"TKT-1", "TKT-2"},
		},
		{
			name:  "a denied row is not listed",
			gate:  resolverGate{deny: map[string]bool{"TKT-1": true}},
			world: visibility.WorldOf(store.TrivialScope()),
			want:  []string{"TKT-2"},
		},
		{
			name:  "a denied world lists nothing",
			world: visibility.DeniedWorld(),
			want:  []string{},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := mustResolver(t, tc.gate, visibility.NopRedactor{}, mentionStore(t))
			page, err := r.MentionedBy(context.Background(), tc.world, "REQ-1", "", 10)
			if err != nil {
				t.Fatal(err)
			}
			if got := pageRefs(page); !slices.Equal(got, tc.want) {
				t.Errorf("got %v, want %v", got, tc.want)
			}
			if page.Next != "" || page.Truncated {
				t.Errorf("Next = %q, truncated %v; want done", page.Next, page.Truncated)
			}
		})
	}
}

func TestResolver_MentionedByPages(t *testing.T) {
	st := memstore.New()
	var want []string
	for i := range 7 {
		id := fmt.Sprintf("TKT-%d", i)
		want = append(want, id)
		if err := st.CreateEntity(context.Background(),
			&entity.Entity{ID: id, Type: "ticket", Content: "`REQ-1`"}); err != nil {
			t.Fatal(err)
		}
	}
	r := mustResolver(t, resolverGate{}, visibility.NopRedactor{}, st)
	w := visibility.WorldOf(store.TrivialScope())

	var got []string
	var after string
	for range 10 {
		page, err := r.MentionedBy(context.Background(), w, "REQ-1", after, 3)
		if err != nil {
			t.Fatal(err)
		}
		got = append(got, pageRefs(page)...)
		if page.Next == "" {
			break
		}
		after = page.Next
	}
	if !slices.Equal(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

// TestResolver_MentionedByBudget pins that a target mentioned only by rows
// the principal cannot read costs one bounded scan per call, and that the
// answer names none of them: no cursor, only the truncated flag.
func TestResolver_MentionedByBudget(t *testing.T) {
	// The index lists rows the store does not hold, which the resolver
	// drops exactly as it drops rows the gate refuses. Seeding 10,600 real
	// rows is quadratic in memstore and too slow under -race.
	refs := make([]entity.Ref, 0, 10600)
	for i := range 10600 {
		refs = append(refs, entity.Ref{ID: fmt.Sprintf("TKT-%05d", i)})
	}
	counting := storetest.NewCounting(fixedIndex{Store: memstore.New(), refs: refs})
	r := mustResolver(t, resolverGate{}, visibility.NopRedactor{}, counting)
	page, err := r.MentionedBy(context.Background(), visibility.WorldOf(store.TrivialScope()), "REQ-1", "", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Items) != 0 || page.Next != "" || !page.Truncated {
		t.Fatalf("got %d items, next %q, truncated %v; want none, no cursor, truncated",
			len(page.Items), page.Next, page.Truncated)
	}
	// Pages of 500 with each page's last id held back for the next, so 499
	// new rows per call: 21 calls to pass the budget of 10000.
	if n := counting.Calls()["MentionSources"]; n != 21 {
		t.Errorf("MentionSources calls = %d, want 21", n)
	}
}

// TestResolver_MentionedByPagesPastHiddenRun: readable sources on both sides
// of a run of unreadable ones are all reached, and the cursor between them
// is the served id.
func TestResolver_MentionedByPagesPastHiddenRun(t *testing.T) {
	st := memstore.New()
	deny := map[string]bool{}
	seedMention(t, st, "TKT-0000")
	for i := 1; i < 1500; i++ {
		id := fmt.Sprintf("TKT-%04d", i)
		deny[id] = true
		seedMention(t, st, id)
	}
	seedMention(t, st, "TKT-9999")
	r := mustResolver(t, resolverGate{deny: deny}, visibility.NopRedactor{}, st)
	w := visibility.WorldOf(store.TrivialScope())

	first, err := r.MentionedBy(context.Background(), w, "REQ-1", "", 1)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(pageRefs(first), []string{"TKT-0000"}) || first.Next != "TKT-0000" {
		t.Fatalf("first page %v, next %q; want TKT-0000 and a cursor on it", pageRefs(first), first.Next)
	}
	second, err := r.MentionedBy(context.Background(), w, "REQ-1", first.Next, 1)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(pageRefs(second), []string{"TKT-9999"}) || second.Next != "" || second.Truncated {
		t.Errorf("second page %v, next %q, truncated %v; want TKT-9999 and done",
			pageRefs(second), second.Next, second.Truncated)
	}
}

// TestResolver_MentionedByExactPage: a page that holds the last source has
// no cursor, and an entity is not its own backlink.
func TestResolver_MentionedByExactPage(t *testing.T) {
	st := memstore.New()
	for _, id := range []string{"REQ-1", "TKT-1", "TKT-2"} {
		seedMention(t, st, id)
	}
	r := mustResolver(t, resolverGate{}, visibility.NopRedactor{}, st)
	page, err := r.MentionedBy(context.Background(), visibility.WorldOf(store.TrivialScope()), "REQ-1", "", 2)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(pageRefs(page), []string{"TKT-1", "TKT-2"}) || page.Next != "" {
		t.Errorf("got %v, next %q; want TKT-1 TKT-2 and no cursor", pageRefs(page), page.Next)
	}
}

func seedMention(t *testing.T, st store.Store, id string) {
	t.Helper()
	if err := st.CreateEntity(context.Background(),
		&entity.Entity{ID: id, Type: "ticket", Content: "`REQ-1`"}); err != nil {
		t.Fatal(err)
	}
}

// TestResolver_MentionedByCursorNamesNoHiddenFace: a full page ends on an
// entity whose hidden face also mentions the target. The cursor is the bare
// id, so it does not reveal that face.
func TestResolver_MentionedByCursorNamesNoHiddenFace(t *testing.T) {
	st := memstore.New()
	for _, e := range []*entity.Entity{
		{ID: "POL-1", Type: "policy", Face: faceDraft, Content: "`REQ-1`"},
		{ID: "POL-1", Type: "policy", Face: facePublished, Content: "`REQ-1`"},
		{ID: "POL-2", Type: "policy", Face: facePublished, Content: "`REQ-1`"},
	} {
		if err := st.CreateEntity(context.Background(), e); err != nil {
			t.Fatal(err)
		}
	}
	gate := resolverGate{faces: map[string]visibility.FaceSet{"policy": visibility.SomeFaces(facePublished)}}
	r := mustResolver(t, gate, visibility.NopRedactor{}, st)
	w := visibility.WorldOf(publishedWorld())
	page, err := r.MentionedBy(context.Background(), w, "REQ-1", "", 1)
	if err != nil {
		t.Fatal(err)
	}
	if page.Next != "POL-1" {
		t.Errorf("Next = %q, want the bare id POL-1", page.Next)
	}
	rest, err := r.MentionedBy(context.Background(), w, "REQ-1", page.Next, 5)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(pageRefs(rest), []string{"POL-2@published"}) {
		t.Errorf("after POL-1: %v, want only POL-2@published", pageRefs(rest))
	}
}

func TestResolver_MentionedByRejects(t *testing.T) {
	r := mustResolver(t, resolverGate{}, visibility.NopRedactor{}, mentionStore(t))
	w := visibility.WorldOf(store.TrivialScope())
	if _, err := r.MentionedBy(context.Background(), w, "REQ-1", "", 0); !errors.Is(err, store.ErrInvalidQuery) {
		t.Errorf("limit 0: err = %v, want ErrInvalidQuery", err)
	}
	noIndex := mustResolver(t, resolverGate{}, visibility.NopRedactor{}, getterOnly{mentionStore(t)})
	if _, err := noIndex.MentionedBy(context.Background(), w, "REQ-1", "", 1); !errors.Is(err, visibility.ErrMentionsUnsupported) {
		t.Errorf("no index: err = %v, want ErrMentionsUnsupported", err)
	}
	if _, err := r.MentionedBy(context.Background(), w, "REQ-1", "not an id", 1); !errors.Is(err, visibility.ErrBadMentionCursor) {
		t.Errorf("bad cursor: err = %v, want ErrBadMentionCursor", err)
	}
}

// getterOnly hides every optional capability of the store it wraps.
type getterOnly struct{ visibility.Loader }

// TestResolver_MentionedByFaults pins that an index fault is returned, never
// read as "no backlinks", and that a gate fault hides sources.
func TestResolver_MentionedByFaults(t *testing.T) {
	w := visibility.WorldOf(store.TrivialScope())
	boom := errors.New("boom")
	failing := mustResolver(t, resolverGate{}, visibility.NopRedactor{}, failingIndex{mentionStore(t), boom})
	if _, err := failing.MentionedBy(context.Background(), w, "REQ-1", "", 5); !errors.Is(err, boom) {
		t.Errorf("index fault: err = %v, want boom", err)
	}
	// A gate fault fails closed, as ResolveIDsErr documents: the type's
	// sources are hidden and the fault is logged.
	gateDown := mustResolver(t, resolverGate{err: boom}, visibility.NopRedactor{}, mentionStore(t))
	page, err := gateDown.MentionedBy(context.Background(), w, "REQ-1", "", 5)
	if err != nil || len(page.Items) != 0 {
		t.Errorf("gate fault: items %v, err %v; want none and no error", pageRefs(page), err)
	}
}

// failingIndex is a store whose mention index always fails.
// fixedIndex answers MentionSources from refs, sorted, for any target.
type fixedIndex struct {
	store.Store
	refs []entity.Ref
}

func (f fixedIndex) MentionSources(_ context.Context, _ string, after entity.Ref, limit int) ([]entity.Ref, error) {
	i, _ := slices.BinarySearchFunc(f.refs, after, store.CompareRefs)
	if i < len(f.refs) && store.CompareRefs(f.refs[i], after) == 0 {
		i++
	}
	return f.refs[i:min(i+limit, len(f.refs))], nil
}

type failingIndex struct {
	visibility.Loader
	err error
}

func (f failingIndex) MentionSources(context.Context, string, entity.Ref, int) ([]entity.Ref, error) {
	return nil, f.err
}

func TestParseMentionCursor(t *testing.T) {
	for _, tc := range []struct {
		in, want string
		ok       bool
	}{
		{"", "", true},
		{"TKT-1", "TKT-1", true},
		{"POL-1@draft", "", false},
		{"not an id", "", false},
	} {
		got, err := visibility.ParseMentionCursor(tc.in)
		if tc.ok != (err == nil) || got != tc.want {
			t.Errorf("ParseMentionCursor(%q) = %q, %v", tc.in, got, err)
		}
		if !tc.ok && !errors.Is(err, visibility.ErrBadMentionCursor) {
			t.Errorf("ParseMentionCursor(%q): err = %v, want ErrBadMentionCursor", tc.in, err)
		}
	}
}

// TestReaders_MentionedBy covers the script readers' forwards: the
// unrestricted reader lists every source, the deny reader refuses.
func TestReaders_MentionedBy(t *testing.T) {
	ctx := context.Background()
	page, err := visibility.Unrestricted(mentionStore(t)).WithWorld(visibility.WorldOf(store.TrivialScope())).
		MentionedBy(ctx, "REQ-1", "", 10)
	if err != nil || !slices.Contains(pageRefs(page), "TKT-1") {
		t.Errorf("unrestricted: %v, %v", pageRefs(page), err)
	}
	if _, err := (visibility.DenyReader{}).MentionedBy(ctx, "REQ-1", "", 10); !errors.Is(err, visibility.ErrReaderUnavailable) {
		t.Errorf("deny reader: err = %v, want ErrReaderUnavailable", err)
	}
}
