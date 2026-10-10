package dataentry

import (
	"context"
	"errors"
	"iter"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/search"
	"github.com/Sourcehaven-BV/rela/internal/search/searchparser"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
	"github.com/Sourcehaven-BV/rela/internal/visibility"
)

// scopeRecordingSearcher records the scope it was handed and serves one hit.
type scopeRecordingSearcher struct {
	scope  map[string]search.TypeScope
	served bool
}

func (s *scopeRecordingSearcher) SearchVisible(
	_ context.Context, _ search.Query, scope map[string]search.TypeScope,
) iter.Seq2[search.Hit, error] {
	return func(yield func(search.Hit, error) bool) {
		s.scope = scope
		s.served = true
		yield(search.Hit{ID: "TKT-001", Type: "ticket"}, nil)
	}
}

// scopeResolver answers SearchFieldScope with a fixed scope or error.
type scopeResolver struct {
	fs  *search.FieldScope
	err error
}

func (r scopeResolver) SearchFieldScope(context.Context, string) (*search.FieldScope, error) {
	return r.fs, r.err
}

func runFreeText(t *testing.T, vs search.VisibleSearcher, res visibility.FieldScoper) error {
	t.Helper()
	q := &queryService{
		visibleSearcher: func() search.VisibleSearcher { return vs },
		fields:          func() visibility.FieldScoper { return res },
	}
	sq := searchparser.ParseQuery("anything")
	scope := map[string]search.TypeScope{"ticket": {AllowAll: true}}
	_, err := q.runVisibleFreeTextSearch(context.Background(), Services{Store: memstore.New()}, sq, scope)
	return err
}

// TestFreeTextSearch_HandsTheFieldScopeToTheSearcher pins the wiring: the
// free-text branch passes the field resolver's scope to the searcher with
// the row scope, so the backend drops hidden-field-only matches.
func TestFreeTextSearch_HandsTheFieldScopeToTheSearcher(t *testing.T) {
	t.Parallel()
	vs := &scopeRecordingSearcher{}
	fs := &search.FieldScope{Gated: []string{"secret"}}
	if err := runFreeText(t, vs, scopeResolver{fs: fs}); err != nil {
		t.Fatalf("search: %v", err)
	}
	if got := vs.scope["ticket"].Fields; got != fs {
		t.Errorf("searcher got field scope %+v, want the resolver's %+v", got, fs)
	}
}

// TestFreeTextSearch_FieldScopeErrorFailsClosed: a resolver that cannot
// compile a field scope must stop the search before the backend runs, as an
// ACL failure (errACLListQuery), never serve unchecked hits.
func TestFreeTextSearch_FieldScopeErrorFailsClosed(t *testing.T) {
	t.Parallel()
	vs := &scopeRecordingSearcher{}
	err := runFreeText(t, vs, scopeResolver{err: errors.New("no request")})
	if !errors.Is(err, errACLListQuery) || !errors.Is(err, search.ErrScope) {
		t.Errorf("err = %v, want errACLListQuery wrapping search.ErrScope", err)
	}
	if vs.served {
		t.Error("the backend ran despite the field scope failure")
	}
}

// TestSearchFieldScoper_FollowsTheResolver pins the search form of each
// resolver kind. A resolver the data-entry search does not know refuses
// every search, so a stub that hides fields cannot pair with a search that
// hides nothing.
func TestSearchFieldScoper_FollowsTheResolver(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	if fs, err := searchFieldScoper(NopFieldVerdictResolver{}).SearchFieldScope(ctx, "ticket"); fs != nil || err != nil {
		t.Errorf("nop: scope=%+v err=%v, want nothing hidden", fs, err)
	}
	fs, err := searchFieldScoper(DemoFieldVerdictResolver{}).SearchFieldScope(ctx, "ticket")
	if err != nil || fs == nil || !fs.Gates("priority") {
		t.Errorf("demo: scope=%+v err=%v, want priority gated on ticket", fs, err)
	}
	unknown := fakeResolver{fv: FieldVerdicts{Visible: map[string]bool{"title": false}}}
	if _, err := searchFieldScoper(unknown).SearchFieldScope(ctx, "ticket"); err == nil {
		t.Error("an unknown resolver kind must refuse, not hide nothing")
	}
}

// TestFreeTextSearch_DemoProfileHidesPriority: the demo profile runs with no
// ACL, so the row scope is the wildcard. Its field scope must still reach
// the backend, so a match on the hidden priority alone is not returned.
func TestFreeTextSearch_DemoProfileHidesPriority(t *testing.T) {
	app := newTestAppV1(t)
	app.fieldResolver = DemoFieldVerdictResolver{}
	seedEntity(app, &entity.Entity{
		ID: "TKT-001", Type: "ticket",
		Properties: map[string]any{"title": "visible words", "priority": "zzhiddenprio"},
	})

	got, err := app.queries.executeQuery(context.Background(), "zzhiddenprio")
	if err != nil {
		t.Fatalf("executeQuery: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("a match on the demo-hidden priority returned %d hits", len(got))
	}
	got, err = app.queries.executeQuery(context.Background(), "visible")
	if err != nil {
		t.Fatalf("executeQuery: %v", err)
	}
	if len(got) != 1 {
		t.Errorf("a title match returned %d hits, want 1", len(got))
	}
}
