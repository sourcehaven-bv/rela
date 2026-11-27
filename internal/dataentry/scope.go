package dataentry

import (
	"encoding/json"
	"net/http"
	"net/url"
	"slices"
	"strings"

	v1 "github.com/Sourcehaven-BV/rela/internal/apiwire/v1"
	entityPkg "github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// ScopeDescriptor encodes the query that defines an ordered result set the
// user is navigating — a typed list, a search result, and (later) other
// sources. It rides on the wire as a single URL-encoded JSON `scope` param,
// unsigned: every field it carries is already freely issuable by the client
// against the list endpoint, so there is nothing to protect against tamper.
// Correctness comes from strict decoding in scopeFromParam, not from a
// signature. See issue #844 and docs/data-entry/api-reference.md.
//
// Filters carries the same flat bracket-format keys the SPA already emits via
// filterStateToApiParams ("filter[status]", "filter[due][gte]", …). Reusing
// that wire format keeps a single source of truth for filter serialization
// and lets the descriptor rebuild a url.Values that the shared list pipeline
// consumes verbatim.
type ScopeDescriptor struct {
	Source  string            `json:"source"`            // "list" | "search" | "pile"
	Type    string            `json:"type"`              // entity type name (singular)
	Filters map[string]string `json:"filters,omitempty"` // filter[...] bracket keys → value

	// ListQuery is the list part of the scope. Its fields are promoted and
	// sit at the top level of the JSON.
	ListQuery

	// Pile is the pile a `source: pile` scope walks (TKT-K3RJLH). It is the
	// only field such a scope carries.
	Pile string `json:"pile,omitempty"`
}

// ListQuery is the part of a list read that a scope repeats, so prev/next
// walks the rows the list showed: its sort, its text query and the
// narrowing it applied. The SPA's ListParams and ScopeDescriptor carry the
// same fields through ListQueryScope (frontend/src/types/entity.ts).
type ListQuery struct {
	Sort string `json:"sort,omitempty"` // "-created,title" form
	Q    string `json:"q,omitempty"`    // free-text query

	// QueryScope is the originating view's `query_scope:`, carried so
	// prev/next walks the set the list actually showed.
	//
	// Without it the position resolves against the entity type's DEFAULT
	// scope regardless of what was on screen: a list showing `archief`
	// would navigate the non-archived set, so the very entity the reader
	// has open is absent from its own scope and the endpoint answers 404
	// not_in_scope. Empty means the default, which is correct for a list
	// that named no scope.
	QueryScope string `json:"query_scope,omitempty"`

	// ScopePage, ScopeTab and Anchor carry the entity-page tab a list was
	// shown in (pagescope.go), for the same reason as QueryScope: prev/next
	// walks the rows the tab showed, not the whole type. All three or none.
	ScopePage string `json:"scope_page,omitempty"`
	ScopeTab  string `json:"scope_tab,omitempty"`
	Anchor    string `json:"anchor,omitempty"`

	// ListID names the configured list the scope walks, so its `condition:`
	// narrows the set as it does on the list's own page (TKT-M07C97Y). Like
	// the list endpoint's `list_id`, it is an id and never an expression.
	// Only with source list. Empty means no list is named, and no condition
	// applies.
	ListID string `json:"list_id,omitempty"`
}

// knownScopeSources gates Source. Extending scope to a new origin is a
// deliberate change: add the source here and wire whatever produces it. An
// unknown source is rejected rather than silently treated as a plain list, so
// a typo in the SPA surfaces as a 400 instead of a wrong result set.
var knownScopeSources = map[string]struct{}{
	"list":   {},
	"search": {},
	"pile":   {},
}

// scopeFromParam decodes and validates the URL-encoded JSON `scope` param.
// The decoded contents are untrusted input: Type must exist in the metamodel,
// Source must be known, and Filters keys must use the filter[...] grammar.
// Returns a human-meaningful reason on rejection (surfaced as the 400 detail).
func scopeFromParam(raw string, meta entityTypeChecker) (scope ScopeDescriptor, ok bool, reason string) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ScopeDescriptor{}, false, "scope is required"
	}

	var d ScopeDescriptor
	if err := json.Unmarshal([]byte(raw), &d); err != nil {
		return ScopeDescriptor{}, false, "scope is not valid JSON: " + err.Error()
	}

	if _, ok := knownScopeSources[d.Source]; !ok {
		return ScopeDescriptor{}, false, "unknown scope source: " + d.Source
	}

	switch d.Source {
	case "pile":
		if d.Pile == "" {
			return ScopeDescriptor{}, false, "scope pile is required for source pile"
		}
		if d.Type != "" || len(d.Filters) > 0 || d.Sort != "" || d.Q != "" || d.QueryScope != "" ||
			d.ScopePage != "" || d.ScopeTab != "" || d.Anchor != "" || d.ListID != "" {

			return ScopeDescriptor{}, false, "a pile scope takes only pile"
		}
		return d, true, ""
	case "list":
		if d.Type == "" {
			return ScopeDescriptor{}, false, "scope type is required"
		}
		if d.Pile != "" {
			return ScopeDescriptor{}, false, "pile needs source pile"
		}
	case "search":
		if strings.TrimSpace(d.Q) == "" {
			return ScopeDescriptor{}, false, "scope q is required for search"
		}
		if d.Pile != "" {
			return ScopeDescriptor{}, false, "pile needs source pile"
		}
		if d.ScopePage != "" || d.ScopeTab != "" || d.Anchor != "" {
			return ScopeDescriptor{}, false, "a page scope needs source list"
		}
		if d.ListID != "" {
			return ScopeDescriptor{}, false, "list_id needs source list"
		}
	}
	if d.Type != "" && !meta.HasEntityType(d.Type) {
		return ScopeDescriptor{}, false, "unknown entity type: " + d.Type
	}

	for key := range d.Filters {
		if !strings.HasPrefix(key, "filter[") || !strings.HasSuffix(key, "]") {
			return ScopeDescriptor{}, false, "invalid filter key: " + key
		}
	}

	return d, true, ""
}

// entityTypeChecker is the narrow capability scopeFromParam needs: confirm a
// type name exists in the metamodel. Declared at the consumer per the
// call-site-interface rule in CLAUDE.md.
type entityTypeChecker interface {
	HasEntityType(name string) bool
}

// toQuery rebuilds the url.Values the shared list pipeline consumes. The keys
// match exactly what handleV1ListEntities reads from r.URL.Query(), so a scope
// produces the same filtered/sorted ordering as the originating list.
func (d ScopeDescriptor) toQuery() url.Values {
	q := url.Values{}
	for key, val := range d.Filters {
		q.Set(key, val)
	}
	if d.Sort != "" {
		q.Set("sort", d.Sort)
	}
	if d.Q != "" {
		q.Set("q", d.Q)
	}
	if d.QueryScope != "" {
		q.Set(QueryScopeParam, d.QueryScope)
	}
	if d.ListID != "" {
		q.Set(listIDParam, d.ListID)
	}
	if d.ScopePage != "" || d.ScopeTab != "" || d.Anchor != "" {
		q.Set(scopePageParam, d.ScopePage)
		q.Set(scopeTabParam, d.ScopeTab)
		q.Set(scopeAnchorParam, d.Anchor)
	}
	return q
}

// storePosition answers a list-sourced position from the store when the
// scope is one the list page itself would serve from the store (TKT-U9DYW4).
// handled=false means "take the Go path", which is the ordinary case for a
// search scope (relevance ordered, mixed type — not a GraphQuery at all) and
// for any list shape the pushdown declines. handled=true with a nil position
// means the response — an error — has been written.
//
// The ACL is honored exactly as on the list page: the plan's query IS the
// principal's compiled read query, so Total, Prev and Next describe visible
// rows only, and an id outside them is the same not_in_scope 404.
//
// A function, not a method: App is at its method load line.
func storePosition(
	a *App, w http.ResponseWriter, r *http.Request, scope ScopeDescriptor, id string,
) (*v1.Position, bool) {
	if scope.Source != "list" {
		return nil, false
	}
	query := scope.toQuery()
	state := a.State()
	gated, err := gateAppListQuery(r.Context(), a, state, scope.Type, query)
	if err != nil {
		return nil, false
	}
	ctx, n, err := resolveListNarrowing(r.Context(), a, state, scope.Type, query)
	if err != nil {
		return nil, false
	}
	plan, empty, ok := n.pushdownPlan(ctx, scope.Type, gated, 1, 1)
	if !ok {
		return nil, false
	}
	var sp store.Position
	found := false
	if !empty {
		if sp, found, err = plan.position(ctx, a.Services().Store, id); err != nil {
			writeListPipelineError(w, r, err)
			return nil, true
		}
	}
	if !found {
		writeV1Error(w, r, http.StatusNotFound, "not_in_scope", "Entity not found in scope", "")
		return nil, true
	}
	return positionFromStore(sp), true
}

// searchStorePosition answers a search-sourced position from the store when
// the scope's query is a structured search over one type that the store can
// answer exactly (searchPlan, TKT-D3RAB0E). handled=false means "take the Go
// path", which reads the whole result; handled=true with a nil position
// means the response, an error or a 404, has been written.
//
// The plan's query is the principal's read query with the search filters
// and sort ANDed on, so Total, Prev and Next describe readable rows only,
// exactly as the Go path's executeQuery result does.
func searchStorePosition(
	a *App, w http.ResponseWriter, r *http.Request, scope ScopeDescriptor, id string,
) (*v1.Position, bool) {
	if scope.Source != "search" {
		return nil, false
	}
	sp, found, handled, err := a.queries.searchPosition(r.Context(), scope.Q, scope.Type, id)
	switch {
	case !handled:
		return nil, false
	case err != nil:
		writeListPipelineError(w, r, err)
		return nil, true
	case !found:
		writeV1Error(w, r, http.StatusNotFound, "not_in_scope", "Entity not found in scope", "")
		return nil, true
	}
	return positionFromStore(sp), true
}

// positionFromStore is the wire form of a store position. A list or search
// neighbor links to the bare id, as scopePositionRef does.
func positionFromStore(sp store.Position) *v1.Position {
	pos := &v1.Position{Current: sp.Index, Total: sp.Total}
	if sp.Prev != nil {
		pos.Prev = &v1.PositionRef{ID: sp.Prev.ID, Type: sp.Prev.Type, Address: sp.Prev.ID}
	}
	if sp.Next != nil {
		pos.Next = &v1.PositionRef{ID: sp.Next.ID, Type: sp.Next.Type, Address: sp.Next.ID}
	}
	return pos
}

// scopeIndex finds id in entities. A list or search scope matches the bare
// id, as it always has. A pile holds faces, so a pile scope matches a named
// face exactly and a bare id at its first row.
func scopeIndex(scope ScopeDescriptor, entities []*entityPkg.Entity, id string) int {
	if scope.Source != "pile" {
		return slices.IndexFunc(entities, func(e *entityPkg.Entity) bool { return e.ID == id })
	}
	ref, err := entityPkg.ParseRef(id)
	if err != nil {
		return -1
	}
	return slices.IndexFunc(entities, func(e *entityPkg.Entity) bool {
		return e.ID == ref.ID && (ref.Face.IsImplicit() || e.Face == ref.Face)
	})
}

// scopePositionRef is the wire neighbor for e. A pile neighbor links to
// the face the pile serves; a list or search neighbor links to the bare
// id, which the world resolves as the page that produced the scope did.
func scopePositionRef(scope ScopeDescriptor, e *entityPkg.Entity) *v1.PositionRef {
	addr := e.ID
	if scope.Source == "pile" {
		addr = entityPkg.FormatStateRef(e.ID, e.Face)
	}
	return &v1.PositionRef{ID: e.ID, Type: e.Type, Address: addr}
}
