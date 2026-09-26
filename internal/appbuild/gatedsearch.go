package appbuild

import (
	"context"
	"fmt"
	"iter"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/search"
	"github.com/Sourcehaven-BV/rela/internal/visibility"
)

// gatedSearcher is the full-text search of [Services.GatedReads]: a plain
// [search.Searcher] whose hits are the ones the ctx principal may read.
//
// It applies the same three gates the data-entry search does, in the same
// order: the per-type row scope (pushed into the search as a
// [search.TypeScope]), field redaction (a hit that matched only hidden
// properties is dropped, so search is not a value oracle), and the face
// allowlist (a `type@face` grant withholds hits on other faces).
//
// It exists because a consumer holding only [search.Searcher] cannot apply
// any of this itself, and a post-hoc row check on raw hits still leaks
// hidden values through which rows matched.
type gatedSearcher struct {
	gate     visibility.DeclarativeGate
	redactor visibility.FieldRedactor
	search   search.FieldVisibleSearcher
	types    []string
}

// newGatedSearcher builds the searcher for a configured policy. It refuses
// every search when the wired searcher cannot redact fields, rather than
// serving unredacted hits.
func newGatedSearcher(
	visible search.VisibleSearcher, gate visibility.DeclarativeGate,
	redactor visibility.FieldRedactor, types []string,
) search.Searcher {
	fvs, ok := visible.(search.FieldVisibleSearcher)
	if !ok {
		return refusedSearcher{err: fmt.Errorf(
			"%w: the wired searcher (%T) cannot redact fields", search.ErrScope, visible)}
	}
	return gatedSearcher{gate: gate, redactor: redactor, search: fvs, types: types}
}

func (g gatedSearcher) Search(ctx context.Context, q search.Query) iter.Seq2[search.Hit, error] {
	return func(yield func(search.Hit, error) bool) {
		bound, err := g.gate.Bind(ctx)
		if err != nil {
			yield(search.Hit{}, err)
			return
		}
		scope, err := g.scope(bound, q.Types)
		if err != nil {
			yield(search.Hit{}, err)
			return
		}
		for hit, err := range g.search.SearchVisibleFields(bound, q, scope, g.hiddenFields) {
			if err == nil && !visibility.FaceAllowed(bound, g.gate, hit.Type, hit.Face) {
				continue
			}
			if !yield(hit, err) {
				return
			}
		}
	}
}

// scope maps each searched type to its read verdict. A denied type is
// absent, which the searcher treats as deny. No wildcard is emitted: a type
// outside the metamodel cannot be granted, so it is not searchable either.
func (g gatedSearcher) scope(ctx context.Context, types []string) (map[string]search.TypeScope, error) {
	if len(types) == 0 {
		types = g.types
	}
	scope := make(map[string]search.TypeScope, len(types))
	for _, typ := range types {
		rq, err := g.gate.ReadQueryFor(ctx, typ)
		if err != nil {
			return nil, err
		}
		switch {
		case rq.AllowAll:
			scope[typ] = search.TypeScope{AllowAll: true}
		case rq.Query != nil:
			scope[typ] = search.TypeScope{Query: rq.Query}
		}
	}
	return scope, nil
}

// hiddenFields reports the properties of e hidden from the ctx principal, in
// the search field vocabulary.
func (g gatedSearcher) hiddenFields(
	ctx context.Context, _ search.Hit, e *entity.Entity,
) (map[string]struct{}, error) {
	hidden := g.redactor.HiddenProperties(ctx, e)
	out := make(map[string]struct{}, len(hidden))
	for name := range hidden {
		out[search.PropFieldPrefix+name] = struct{}{}
	}
	return out, nil
}

// refusedSearcher fails every search with err. It pairs with
// [visibility.DenyReader]: when gated search cannot be built, search is
// refused rather than served ungated.
type refusedSearcher struct{ err error }

func (r refusedSearcher) Search(context.Context, search.Query) iter.Seq2[search.Hit, error] {
	return func(yield func(search.Hit, error) bool) { yield(search.Hit{}, r.err) }
}
