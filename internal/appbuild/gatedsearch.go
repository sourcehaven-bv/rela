package appbuild

import (
	"context"
	"errors"
	"fmt"
	"iter"
	"log/slog"
	"slices"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/search"
	"github.com/Sourcehaven-BV/rela/internal/visibility"
)

// gatedSearcher is the search half of [Services.GatedReads]: a plain
// [search.Searcher] whose hits are limited to what the ctx principal may read.
//
// It applies the same three filters the data-entry `/_search` pipeline does
// (TKT-BA8BSX, TKT-GGQ0JT, TKT-O7R2A1), resolved per call from the ctx
// principal:
//
//   - the row scope, from the per-type ACL read query, so a hidden entity is
//     never a hit;
//   - the field filter, which drops a hit that matched only `visible:`-hidden
//     properties (the match-on-hidden-field oracle);
//   - the face filter, which drops a hit on a content state the principal's
//     grant withholds.
//
// q.Limit counts only hits that pass all three filters. Hit.Title is always
// empty: the index title is unredacted, so a consumer that shows a title must
// read the entity through the gated reader, as MCP's search_entities does.
type gatedSearcher struct {
	visible  search.FieldVisibleSearcher
	gate     visibility.DeclarativeGate
	redactor visibility.FieldRedactor
	meta     *metamodel.Metamodel
}

// newGatedSearcher builds the searcher for [Services.GatedReads]. Under NopACL
// (d == nil) it returns the raw searcher: byte-identical to pre-ACL behavior,
// not a bypass. When a policy exists but the gate cannot be built, it REFUSES
// with an erroring searcher rather than degrading to raw hits (RR-GKCZO5).
func newGatedSearcher(
	raw search.Searcher, visible search.VisibleSearcher, meta *metamodel.Metamodel,
	d *acl.Declarative, redactor visibility.FieldRedactor,
) search.Searcher {
	if d == nil {
		return raw
	}
	fvs, ok := visible.(search.FieldVisibleSearcher)
	if !ok {
		slog.Error("appbuild: wired searcher cannot redact fields; gated search REFUSED",
			"searcher", fmt.Sprintf("%T", visible))
		return search.ErrSearcher(fmt.Errorf("%w: searcher cannot apply field visibility", search.ErrScope))
	}
	gate, err := visibility.NewDeclarativeGate(d)
	if err != nil {
		slog.Error("appbuild: ACL gate unavailable; gated search REFUSED", "err", err)
		return search.ErrSearcher(fmt.Errorf("%w: ACL gate unavailable", search.ErrScope))
	}
	if redactor == nil {
		redactor = visibility.NopRedactor{}
	}
	return gatedSearcher{visible: fvs, gate: gate, redactor: redactor, meta: meta}
}

// Search implements [search.Searcher].
func (g gatedSearcher) Search(ctx context.Context, q search.Query) iter.Seq2[search.Hit, error] {
	return func(yield func(search.Hit, error) bool) {
		bound, err := g.gate.Bind(ctx)
		if err != nil {
			yield(search.Hit{}, fmt.Errorf("%w: %w", search.ErrScope, err))
			return
		}
		scope, err := g.scope(bound, q.Types)
		if err != nil {
			yield(search.Hit{}, err)
			return
		}
		if len(scope) == 0 {
			return // nothing readable; do not touch the backend
		}
		// The face filter runs here, after the inner searcher, so the limit
		// is applied last rather than by the inner searcher.
		inner := q
		inner.Limit = 0
		var hits iter.Seq2[search.Hit, error]
		if _, nop := g.redactor.(visibility.NopRedactor); nop {
			hits = g.visible.SearchVisible(bound, inner, scope)
		} else {
			hits = g.visible.SearchVisibleFields(bound, inner, scope, g.hiddenFields)
		}
		emitted := 0
		for h, err := range hits {
			if err != nil {
				yield(search.Hit{}, err)
				return
			}
			if !visibility.FaceAllowed(bound, g.gate, h.Type, h.Face) {
				continue
			}
			h.Title = ""
			if !yield(h, nil) {
				return
			}
			emitted++
			if q.Limit > 0 && emitted >= q.Limit {
				return
			}
		}
	}
}

// scope maps the per-type ACL read verdicts onto the search scope shape.
// DenyAll types are absent, which the seam treats as deny. No wildcard entry
// is emitted, so a type outside the metamodel is never visible. When the
// query names types, only those are resolved.
func (g gatedSearcher) scope(ctx context.Context, only []string) (map[string]search.TypeScope, error) {
	types := make([]string, 0, len(g.meta.Entities))
	for name := range g.meta.Entities {
		if len(only) == 0 || slices.Contains(only, name) {
			types = append(types, name)
		}
	}
	slices.Sort(types)

	scope := make(map[string]search.TypeScope, len(types))
	for _, typ := range types {
		rqr, err := g.gate.ReadQueryFor(ctx, typ)
		if err != nil {
			return nil, fmt.Errorf("%w: read scope for %q: %w", search.ErrScope, typ, err)
		}
		switch {
		case rqr.AllowAll:
			scope[typ] = search.TypeScope{AllowAll: true}
		case rqr.Query != nil:
			scope[typ] = search.TypeScope{Query: rqr.Query}
		}
	}
	return scope, nil
}

// hiddenFields is the [search.HiddenFieldsFunc]: the redactor's hidden
// property names, qualified into the search field vocabulary.
func (g gatedSearcher) hiddenFields(
	ctx context.Context, _ search.Hit, e *entity.Entity,
) (map[string]struct{}, error) {
	if e == nil {
		return nil, errors.New("appbuild: gated search: nil entity")
	}
	hidden := g.redactor.HiddenProperties(ctx, e)
	out := make(map[string]struct{}, len(hidden))
	for name := range hidden {
		out[search.PropFieldPrefix+name] = struct{}{}
	}
	return out, nil
}
