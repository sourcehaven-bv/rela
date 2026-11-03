package visibility

import (
	"context"
	"errors"
	"fmt"
	"iter"
	"maps"
	"slices"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/search"
)

// MaxSearchLimit caps the hits one gated search may return, so a
// caller-chosen unlimited query cannot turn one request into an unbounded
// scan. The value matches the data-entry free-text cap.
const MaxSearchLimit = 1000

// SearchScoper is the policy half of a [Searcher]: it binds one per-operation
// ACL scope and answers the per-type list-read verdict and face set.
// [DeclarativeGate] is the production implementation.
type SearchScoper interface {
	RowGate
	FaceGate
	Bind(ctx context.Context) (context.Context, error)
	ReadQueryFor(ctx context.Context, entityType string) (acl.ReadQueryResult, error)
}

// FieldScoper compiles, for the request bound on ctx, which properties a
// text match may count on for one entity type ([search.FieldScope]). A nil
// scope hides nothing. [PolicyRedactor] is the production implementation.
//
// It is called once per type per search, never per hit: the scope is applied
// by the search backend to every row at once.
type FieldScoper interface {
	SearchFieldScope(ctx context.Context, entityType string) (*search.FieldScope, error)
}

// Searcher is a [search.Searcher] whose hits are gated for the principal on
// ctx at call time. It resolves the row scope from the policy and then runs
// [SearchScoped]:
//
//   - Row scope: each type's list-read verdict becomes a [search.TypeScope].
//     A type without a grant is absent from the scope, which denies it. No
//     wildcard is emitted, so an entity whose type is not in the metamodel is
//     not searchable under a policy.
//   - Field scope and face scope: see [SearchScoped].
//
// [search.Hit.Title] is cleared on every hit. It is the raw indexed value, and
// a hit that matched on the id or body survives even when the title property
// is hidden. Consumers take the title from the gated reader.
//
// A query with Filters or Sort is refused. The hidden-field check covers the
// free text only, so a filter or sort on a hidden property would reveal its
// value. No gated consumer needs either today.
//
// Every failure is fail-closed: an error is yielded and no further hits are.
type Searcher struct {
	vs     search.VisibleSearcher
	scoper SearchScoper
	fields FieldScoper
	types  []string
}

var _ search.Searcher = (*Searcher)(nil)

// NewSearcher builds a gated searcher. types is the set of metamodel entity
// types a query may reach; a query naming other types matches nothing.
//
// Nil: vs, scoper and fields are rejected — a missing collaborator would
// otherwise surface as ungated or empty results.
func NewSearcher(
	vs search.VisibleSearcher, scoper SearchScoper, fields FieldScoper, types []string,
) (*Searcher, error) {
	if vs == nil {
		return nil, errors.New("visibility: NewSearcher: VisibleSearcher is required")
	}
	if scoper == nil {
		return nil, errors.New("visibility: NewSearcher: SearchScoper is required")
	}
	if fields == nil {
		return nil, errors.New("visibility: NewSearcher: FieldScoper is required")
	}
	return &Searcher{vs: vs, scoper: scoper, fields: fields, types: slices.Clone(types)}, nil
}

// Search implements [search.Searcher].
func (s *Searcher) Search(ctx context.Context, q search.Query) iter.Seq2[search.Hit, error] {
	return func(yield func(search.Hit, error) bool) {
		if len(q.Filters) > 0 || len(q.Sort) > 0 {
			yield(search.Hit{}, fmt.Errorf("%w: property filters and sorting are not supported", search.ErrScope))
			return
		}
		bound, err := s.scoper.Bind(ctx)
		if err != nil {
			yield(search.Hit{}, fmt.Errorf("%w: %w", search.ErrScope, err))
			return
		}
		scope, err := s.scope(bound, q.Types)
		if err != nil {
			yield(search.Hit{}, fmt.Errorf("%w: %w", search.ErrScope, err))
			return
		}
		if len(scope) == 0 {
			return
		}
		if q.Limit <= 0 || q.Limit > MaxSearchLimit {
			q.Limit = MaxSearchLimit
		}
		for h, err := range SearchScoped(bound, s.vs, s.scoper, s.fields, q, scope, s.types) {
			if err != nil {
				yield(search.Hit{}, err)
				return
			}
			h.Title = ""
			if !yield(h, nil) {
				return
			}
		}
	}
}

// scope maps each reachable type's list-read verdict onto a search scope.
// Only the types the query names are consulted, when it names any.
func (s *Searcher) scope(ctx context.Context, want []string) (map[string]search.TypeScope, error) {
	scope := make(map[string]search.TypeScope, len(s.types))
	for _, typ := range s.types {
		if len(want) > 0 && !slices.Contains(want, typ) {
			continue
		}
		rqr, err := s.scoper.ReadQueryFor(ctx, typ)
		if err != nil {
			return nil, err
		}
		switch {
		case rqr.AllowAll:
			scope[typ] = search.TypeScope{AllowAll: true, Faces: rqr.Faces}
		case rqr.Query != nil:
			scope[typ] = search.TypeScope{Query: rqr.Query, Faces: rqr.Faces}
		}
	}
	return scope, nil
}

// SearchScoped runs q under a row scope the caller resolved for the request
// bound on ctx, and adds the rest of the read policy:
//
//   - Field scope: each type's [search.FieldScope] from fields, so a hit
//     whose text matched only `visible:`-hidden properties is dropped (the
//     match-on-hidden-field oracle, TKT-GGQ0JT). The backend applies it to
//     every row at once; nothing calls back into the policy per hit. A
//     [search.WildcardType] entry cannot carry a field scope, so under one
//     each type in types (the metamodel's) is given its own entry with the
//     wildcard's verdict and its field scope. A type outside the metamodel
//     has no `visible:` grants, so the wildcard alone covers it.
//   - Face scope: a hit on a face gate does not admit is dropped. The scope
//     already carries each type's face allowlist and every searcher trims by
//     it before the world ranks, so this is the fail-closed check at the
//     edge.
//
// It is the one gated search path: [Searcher] and the data-entry search both
// run it. Every failure is yielded wrapped in [search.ErrScope], and no
// further hits follow.
func SearchScoped(
	ctx context.Context, vs search.VisibleSearcher, gate RowGate, fields FieldScoper,
	q search.Query, scope map[string]search.TypeScope, types []string,
) iter.Seq2[search.Hit, error] {
	return func(yield func(search.Hit, error) bool) {
		withFields, err := withFieldScopes(ctx, fields, scope, types)
		if err != nil {
			yield(search.Hit{}, err)
			return
		}
		for h, err := range vs.SearchVisible(ctx, q, withFields) {
			if err != nil {
				yield(search.Hit{}, err)
				return
			}
			if !FaceAllowed(ctx, gate, h.Type, h.Face) {
				continue
			}
			if !yield(h, nil) {
				return
			}
		}
	}
}

// withFieldScopes returns a copy of scope with each type's field scope set.
// A field scope the caller already set is kept when the policy hides nothing
// for the type, and refused when the policy has one too; it is never widened.
// Under a wildcard entry, every type in types without an entry of its own
// gets one carrying the wildcard's verdict, so its field scope has a place to
// ride. The row verdict is unchanged: an explicit entry wins over the
// wildcard, and here it says the same thing.
func withFieldScopes(
	ctx context.Context, fields FieldScoper, scope map[string]search.TypeScope, types []string,
) (map[string]search.TypeScope, error) {
	out := make(map[string]search.TypeScope, len(scope))
	maps.Copy(out, scope)
	if wild, ok := scope[search.WildcardType]; ok {
		for _, typ := range types {
			if _, explicit := out[typ]; !explicit {
				out[typ] = wild
			}
		}
	}
	for typ, ts := range out {
		if typ == search.WildcardType {
			continue
		}
		fs, err := fields.SearchFieldScope(ctx, typ)
		if err != nil {
			return nil, fmt.Errorf("%w: field scope for %q: %w", search.ErrScope, typ, err)
		}
		switch {
		case fs == nil:
			// The policy hides nothing here; a scope the caller set stays.
			// Replacing it with nil would widen what the search may match.
			continue
		case ts.Fields != nil:
			// Two scopes would have to be intersected, and nothing composes
			// them yet; refuse rather than drop either.
			return nil, fmt.Errorf("%w: type %q already carries a field scope", search.ErrScope, typ)
		}
		ts.Fields = fs
		out[typ] = ts
	}
	return out, nil
}

// DenySearcher refuses every search with [ErrReaderUnavailable]. It is the
// search counterpart of [DenyReader]: the substitute when a policy is
// configured but the gated [Searcher] could not be built.
type DenySearcher struct{}

var _ search.Searcher = DenySearcher{}

// Search implements [search.Searcher]: always refuses.
func (DenySearcher) Search(context.Context, search.Query) iter.Seq2[search.Hit, error] {
	return func(yield func(search.Hit, error) bool) {
		yield(search.Hit{}, ErrReaderUnavailable)
	}
}
