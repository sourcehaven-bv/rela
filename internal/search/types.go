package search

import (
	"context"
	"errors"
	"fmt"
	"iter"
	"slices"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// Rule is [store.ResolutionRule]: the world-resolution rule that selected the
// face a hit matched on. One vocabulary, shared with worldreader and the
// data-entry wire by aliasing rather than mirroring.
type Rule = store.ResolutionRule

const (
	// RuleUnscoped is rule 1: the world declares no resolution for this
	// entity's type, so its default state was searched.
	RuleUnscoped = store.ResolutionUnscoped
	// RuleChain is rule 2: the first chain coordinate that EXISTS.
	RuleChain = store.ResolutionChain
	// RuleFallbackDefault is rule 3 under `otherwise: default`: no chain
	// coordinate exists, so the default state stood in. This is the case
	// [Hit.IsFallback] reports and callers must label.
	RuleFallbackDefault = store.ResolutionFallbackDefault
)

// Hit is a minimal result from a search operation.
//
// Face and Via carry WORLD PROVENANCE (TKT-9KZGJO): which face of the
// entity actually matched, and by which resolution rule the world chose
// it. See [Hit.IsFallback] for the case a caller must surface.
type Hit struct {
	ID    string
	Type  string
	Title string

	// Face is the coordinate of the face whose text matched. The zero
	// face is the default state. In a world search this is the
	// entity's PRIME — a world resolves at most one face per entity, so
	// there is never more than one hit per entity to disambiguate.
	Face entity.Face

	// Via records which resolution rule selected the matched face,
	// mirroring worldreader.Rule.
	//
	// It is NOT sufficient on its own to answer "is this a substitute" —
	// see [Hit.ChainPosition] and [Hit.IsFallback].
	Via Rule

	// ChainPosition is the 0-based rank of the matched coordinate within
	// the world's chain for this entity's type. Meaningful only when Via
	// is [RuleChain].
	//
	// # Why the rule alone is not enough
	//
	// [RuleChain] means SOME coordinate the world selects exists — never
	// WHICH. Under `select: [published, draft]` a genuine published match
	// and a draft standing in for a missing published face both report
	// RuleChain identically, so a `published`-world searcher shown a hit
	// whose displayed text lacks the term had no way to know why.
	//
	// That gap was found on the HTTP surface first (a draft rendered under
	// ?world=published, labeled read-only published) and fixed there with
	// the same field, `_world.chain_position`. Search reuses the shape
	// rather than re-deriving it: position 0 is the world's first choice,
	// anything greater is a within-chain substitute.
	ChainPosition int
}

// IsFallback reports whether this hit reached the reader only because the
// world fell past the coordinate it actually asked for.
//
// Callers that render results MUST surface this. A `published`-world search
// that matched a draft face shows the reader a title whose displayed
// (published) text need not contain the term they searched for; unlabeled,
// that reads as a broken search rather than as the accurate statement "no
// published face exists, here is the concept".
//
// BOTH substitute shapes count, which is the correction that makes this
// usable. [RuleFallbackDefault] is the `otherwise:` arm — the chain matched
// NOTHING and the default state stood in. A chain with several candidates
// can also substitute WITHIN itself, and that reports [RuleChain]: under
// `select: [published, draft]` a missing published face resolves to the
// draft. Reporting only the first would leave the second — the exact case
// TKT-9KZGJO's design calls out — silently unlabeled.
// Face is one matched content state: the entity, which of its faces the
// text matched, and by which world-resolution rule that face was chosen.
// It is the backend-level result [Hit] is built from — the seam that lets
// provenance survive from the index to the caller, which a bare ID could
// not carry.
type Face struct {
	ID   string
	Face entity.Face
	Via  Rule

	// ChainPosition is the 0-based rank, within the world's chain for this
	// entity type, of the matched coordinate. Meaningful only when Via is
	// [RuleChain]; zero otherwise. See [Hit.ChainPosition] for why the rule
	// alone cannot answer "was this the world's first choice".
	ChainPosition int
}

func (h Hit) IsFallback() bool { return isFallback(h.Via, h.ChainPosition) }

// IsFallback reports whether this face is a SUBSTITUTE — the same question
// [Hit.IsFallback] answers, asked one layer earlier.
//
// Backends work in Faces and callers in Hits, and both need the verdict.
// Sharing one predicate is what keeps them from drifting: the rule has two
// arms (see [Hit.IsFallback]) and a copy that remembered only the
// `fallback-default` one would be exactly the gap this ticket closed.
func (f Face) IsFallback() bool { return isFallback(f.Via, f.ChainPosition) }

// isFallback is the single definition of "the reader is looking at a
// stand-in". See [Hit.IsFallback] for why both arms are required.
func isFallback(via Rule, position int) bool {
	return via == RuleFallbackDefault || (via == RuleChain && position > 0)
}

// Searcher provides search and filtering over entities. It is a top-level
// service separate from the Store: it builds its state by subscribing to
// store events or by wrapping a Backend. Smart backends (e.g. Postgres)
// can provide native implementations; simple backends use the generic
// implementation in this package.
type Searcher interface {
	Search(ctx context.Context, q Query) iter.Seq2[Hit, error]
}

// Backend is a pluggable full-text search index. It implements
// store.EntityObserver so it can be attached to a store as a write observer,
// and provides a Search method for querying. Implementations must be safe
// for concurrent use. Lifecycle (construction, population on startup,
// close) is the consumer's responsibility — the store does not manage it.
type Backend interface {
	store.EntityObserver

	// Search returns the matching FACES for the query text, ordered by
	// relevance, resolved under the given world. limit ≤ 0 means no
	// limit.
	//
	// A world IS the search scope (TKT-9KZGJO): searching a world means
	// searching each entity's resolved prime in it, so the world's own
	// chain answers "which face do I look at" and no per-world
	// `searchable:` config is needed. `select: [published, draft]` finds
	// a draft-only policy via its draft face; `select: [nl, en]` finds
	// Dutch falling back to English and never French.
	//
	// Because a world resolves at most ONE prime per entity, the result
	// holds at most one Face per entity STRUCTURALLY. That is what makes
	// a limit counting entities fall out for free — no PARTITION BY, no
	// over-fetch-and-group. Implementations must preserve it.
	//
	// The trivial scope ([store.TrivialScope]) must reduce to exactly
	// the pre-worlds query, allocating nothing. An unset scope is
	// [store.ErrInvalidQuery].
	Search(text string, limit int, world store.WorldScope) ([]Face, error)
}

// FieldMatcher is the optional match-provenance capability a Backend may
// implement. MatchedFields reports, for a single already-loaded entity and
// the same query text, which logical fields the match came from. It uses the
// [FieldID] / [FieldContent] / [PropFieldPrefix] vocabulary and the backend's
// own matching rules, so the answer is what Search actually matched. A
// backend without it falls back to [MatchTextFields] (see
// [Service.MatchedFields]).
//
// It answers per entity because it runs only over the hits Search already
// produced.
//
// It must never report a field Search did not match on. A field scope keeps
// a hit when a visible field matched, so an extra field can return a hit
// whose only match is a hidden property. Leaving out a field that did match
// is safe: the hit is dropped, which hides more, never less. When in doubt,
// leave the field out.
type FieldMatcher interface {
	Backend
	MatchedFields(e *entity.Entity, text string) map[string]struct{}
}

// WildcardType is the reserved scope-map key that supplies the default
// verdict for entity types without an explicit entry. It cannot collide
// with a real entity type: metamodel type names are identifiers.
const WildcardType = "*"

// TypeScope is one per-type visibility verdict inside a SearchVisible
// scope map. Exactly one meaning applies:
//
//   - AllowAll true → every entity of the type is visible.
//   - Query non-nil → only entities matching the GraphQuery are visible.
//     The query's EntityType must equal the scope-map key it is stored
//     under; the consumer constructing the scope owns that consistency.
//   - zero value → deny the type (an explicit deny entry is equivalent
//     to the type being absent from the map).
//
// Faces is the face allowlist of the grant: nil admits every face, a
// non-empty list only the faces it names. An empty non-nil list is refused
// by [ValidateScope], because [store.GraphQuery.FaceIn] reads an empty list
// as "every face" and a backend that forwarded it would fail open. It
// applies with either meaning above. Like the
// verdict itself it trims the candidate faces BEFORE the world ranks them,
// so a denied prime falls through to a readable face, as on lists and the
// single-entity read. A [WildcardType] entry must leave it nil.
//
// Fields, when set, gates which properties a text match may count on for
// this type ([FieldScope]). Nil gates nothing. A [WildcardType] entry must
// leave it nil.
//
// The scope map is server-derived (from ACL policy verdicts), never
// wire-supplied.
type TypeScope struct {
	AllowAll bool
	Query    *store.GraphQuery
	Faces    []entity.Face
	Fields   *FieldScope
}

// FieldScope closes the match-on-hidden-field oracle for one type: a search
// must not confirm a property's value by returning an entity whose only match
// is a property the reader may not see.
//
// It is an allowlist. A property p of a row is visible when p is not in Gated,
// or when some grant lists p and the grant's When matches the row. Hidden is
// derived as Gated minus what the grants allow; it is never authored. A hit
// survives when its text matched the id, the content (never property-gated),
// or at least one visible property.
//
// The verdict is per row, and a backend evaluates it inside its own query
// (pgstore) or in one batch per page (the generic [Visible]). Nothing calls
// back into the ACL per row: the consumer compiles the scope once per request.
type FieldScope struct {
	Gated  []string
	Grants []FieldGrant
}

// FieldGrant makes Fields visible on the rows When matches. A nil When
// matches every row. When.EntityType is empty or the scope's type; its Faces
// selection is set by the backend, which evaluates it against the matched
// face row.
type FieldGrant struct {
	Fields []string
	When   *store.GraphQuery
}

// Gates reports whether the scope can hide property prop on any row.
func (fs *FieldScope) Gates(prop string) bool {
	return fs != nil && slices.Contains(fs.Gated, prop)
}

// AdmitsMatch reports whether a hit whose text matched the fields in matched
// survives: it matched the id or the content, or a property the scope shows
// on this row. holds reports whether grant i's When matches the row; it is
// asked only about grants with a non-nil When.
func (fs *FieldScope) AdmitsMatch(matched map[string]struct{}, holds func(i int) bool) bool {
	for f := range matched {
		prop, isProp := strings.CutPrefix(f, PropFieldPrefix)
		if !isProp || !fs.Gates(prop) {
			return true // id, content, or a property no grant decides
		}
		for i, g := range fs.Grants {
			if slices.Contains(g.Fields, prop) && (g.When == nil || holds(i)) {
				return true
			}
		}
	}
	return false
}

// AdmitsFace reports whether the scope's face allowlist admits f.
func (ts TypeScope) AdmitsFace(f entity.Face) bool {
	return ts.Faces == nil || slices.Contains(ts.Faces, f)
}

// ValidateScope rejects the scope-map shapes every [VisibleSearcher]
// refuses: an entry with an empty non-nil face allowlist (see
// [TypeScope.Faces]), and a [WildcardType] entry carrying a Query (a
// GraphQuery targets one entity type) or a face allowlist. The error wraps
// [ErrScope].
func ValidateScope(scope map[string]TypeScope) error {
	for typ, ts := range scope {
		if ts.Faces != nil && len(ts.Faces) == 0 {
			return fmt.Errorf("%w: scope entry %q has an empty face allowlist", ErrScope, typ)
		}
		if ts.Fields == nil {
			continue
		}
		for _, g := range ts.Fields.Grants {
			if g.When != nil && g.When.EntityType != "" && g.When.EntityType != typ {
				return fmt.Errorf("%w: scope entry %q has a field grant on type %q", ErrScope, typ, g.When.EntityType)
			}
		}
	}
	ws, ok := scope[WildcardType]
	if !ok {
		return nil
	}
	if ws.Query != nil {
		return fmt.Errorf("%w: wildcard scope entry cannot carry a GraphQuery", ErrScope)
	}
	if ws.Faces != nil {
		return fmt.Errorf("%w: wildcard scope entry cannot carry a face allowlist", ErrScope)
	}
	if ws.Fields != nil {
		return fmt.Errorf("%w: wildcard scope entry cannot carry a field scope", ErrScope)
	}
	return nil
}

// AdmitFunc trims a batch of candidate faces to the ones a reader may see.
// It returns the admitted subset; an error fails the search closed. A
// backend calls it on whole families BEFORE the world ranks them, so the
// world picks each entity's prime among admitted faces only.
type AdmitFunc func(candidates []Candidate) ([]Candidate, error)

// AdmittingBackend is the optional [Backend] capability a [Service] needs
// to honor [Query.Admit]. SearchAdmitted is [Backend.Search] with admit
// applied to the candidate faces before resolution; a nil admit is exactly
// Search. A Service over a backend without it refuses an admitting query
// rather than ranking unadmitted faces.
type AdmittingBackend interface {
	SearchAdmitted(text string, limit int, world store.WorldScope, admit AdmitFunc) ([]Face, error)
}

// ResolveTypeScope applies the scope lookup rule shared by every
// VisibleSearcher implementation: the exact type entry wins, else the
// reserved [WildcardType] entry, else deny (fail-closed). The second
// return reports whether any entry applied — false means the type is
// denied outright.
//
// Fail-closed is the load-bearing property: a nil or empty scope map
// denies everything, and an entity type the scope builder never saw
// (e.g. removed from the metamodel while its files remain on disk) is
// hidden rather than leaked.
func ResolveTypeScope(scope map[string]TypeScope, entityType string) (TypeScope, bool) {
	if ts, ok := scope[entityType]; ok {
		return ts, ts.AllowAll || ts.Query != nil
	}
	if ts, ok := scope[WildcardType]; ok {
		return ts, ts.AllowAll || ts.Query != nil
	}
	return TypeScope{}, false
}

// VisibleSearcher executes a search restricted to a per-type visibility
// scope. It is the read-side ACL seam for search: the consumer resolves
// ACL verdicts into the scope map (ACL stays at the call site — this
// package never sees a principal or a policy) and the implementation
// guarantees no hit outside the scope is ever yielded, on any backend.
//
// Contract, pinned by storetest.RunVisibleSearchTests (any new
// implementation must pass it):
//
//   - Scope lookup follows [ResolveTypeScope]: exact → "*" → deny.
//   - q.Limit bounds the number of VISIBLE hits — it is applied after
//     visibility filtering, never before. (A pre-visibility limit
//     starves restricted principals: the top-K candidates may all be
//     hidden while visible matches rank below them.)
//   - Relative order of visible hits equals the order the ungated
//     search on the same backend would yield them in.
//   - A [WildcardType] entry carrying a Query or a face allowlist is
//     invalid ([ValidateScope]) and yields an error.
//   - The scope trims candidate faces BEFORE the world ranks them: a hit
//     is the prime of the faces the scope admits, never a prime the scope
//     denies.
//   - q.Sort is ignored, matching [Service.Search].
//   - A type whose entry carries [TypeScope.Fields] yields a hit only when
//     [FieldScope.AdmitsMatch] keeps it, judged on the matched face row. The
//     limit counts these surviving hits only.
//
// Service plus this package's generic wrapper (NewVisible) serve the
// simple backends; smart backends (pgstore) implement it natively by
// composing visibility into the search query itself.
//
// Its iterators follow [store.EntityReader]'s rule: no backend resource is
// held while yielding.
type VisibleSearcher interface {
	SearchVisible(ctx context.Context, q Query, scope map[string]TypeScope) iter.Seq2[Hit, error]
}

// ErrScope marks a SearchVisible failure that occurred while evaluating
// the visibility scope (GraphQuery/MatchingFaces execution), as opposed
// to a plain search-backend failure. Consumers route ErrScope failures
// through their ACL-error path. Implementations that cannot separate
// the two phases (pgstore runs one combined statement) wrap the whole
// failure in ErrScope — the query is the gate there.
var ErrScope = errors.New("search: visibility scope evaluation failed")

// Query describes a search request.
type Query struct {
	Text    string           // free-text search (ranked by relevance when set)
	Types   []string         // filter by entity types
	Filters []PropertyFilter // property-level filters
	Sort    []SortClause     // ordering (ignored when Text is set)
	Limit   int              // max results (0 = no limit)

	// World is the resolution scope: which face of each entity this
	// search looks at. It is required: the unset zero value is
	// [store.ErrInvalidQuery] (see [ValidateQuery]).
	//
	// Limit counts ENTITIES, and that is structural rather than
	// enforced: a world resolves at most one prime per entity, so the
	// result set holds at most one row per entity to begin with.
	World store.WorldScope

	// Admit, when set, trims each entity's candidate faces before World
	// ranks them ([AdmitFunc]). It is server-derived (the visibility
	// wrapper sets it from the ACL scope, and runs a caller's own Admit on
	// what the scope admitted), never wire-supplied. Nil admits every face.
	Admit AdmitFunc
}

// PropertyFilter matches entities by property value.
type PropertyFilter struct {
	Property string
	Value    string
	Op       FilterOp
}

// FilterOp defines how a property filter matches.
type FilterOp int

const (
	FilterEq        FilterOp = iota // exact match (default)
	FilterNe                        // not equal
	FilterContains                  // substring match
	FilterGt                        // greater than
	FilterLt                        // less than
	FilterGte                       // greater than or equal
	FilterLte                       // less than or equal
	FilterIn                        // value is one of a comma-separated set
	FilterExists                    // property is set (Value ignored)
	FilterNotExists                 // property is not set (Value ignored)
)

// SortClause defines a single sort dimension.
type SortClause struct {
	Field     string
	Direction SortDirection
}

// SortDirection is ascending or descending.
type SortDirection int

const (
	SortAsc SortDirection = iota
	SortDesc
)
