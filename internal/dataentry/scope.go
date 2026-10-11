package dataentry

import (
	"context"
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
	Sort    string            `json:"sort,omitempty"`    // "-created,title" form
	Q       string            `json:"q,omitempty"`       // free-text query

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

	// Pile is the pile a `source: pile` scope walks (TKT-K3RJLH). It is the
	// only field such a scope carries.
	Pile string `json:"pile,omitempty"`
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

	// Per-source required fields differ. A list scope is reproduced from
	// {type, filters, sort}, so type is mandatory. A search scope is
	// reproduced from {q, type?}: q is mandatory and type is an *optional*
	// narrowing of a possibly-mixed-type result, so it is validated only
	// when present.
	switch d.Source {
	case "pile":
		// A pile is its own ordered set: nothing else narrows or orders it.
		if d.Pile == "" {
			return ScopeDescriptor{}, false, "scope pile is required for source pile"
		}
		if d.Type != "" || len(d.Filters) > 0 || d.Sort != "" || d.Q != "" || d.QueryScope != "" ||
			d.ScopePage != "" || d.ScopeTab != "" || d.Anchor != "" {

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
		// The search pipeline does not narrow to a page tab, so it would
		// walk more rows than the tab showed. A tab sends source "list".
		if d.ScopePage != "" || d.ScopeTab != "" || d.Anchor != "" {
			return ScopeDescriptor{}, false, "a page scope needs source list"
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

// resolveScope produces the fully ordered entity set a scope refers to,
// dispatching on Source. A list scope runs the shared list pipeline
// (single-type, property-sorted). A search scope runs executeQuery — the same
// relevance-ordered, possibly-mixed-type pipeline the search view uses — then
// narrows to Type when one was supplied. Routing search through executeQuery
// (not scopedSortedEntities) is what makes prev/next correct across mixed-type
// search results: position is found within the exact set the user saw.
//
// Both branches end gated: the list pipeline resolves the read scope
// internally, and executeQuery gates the search pipeline internally
// since TKT-BA8BSX (it routes through search.VisibleSearcher; the
// external readableSubset patch this branch used in the interim is
// retired). Without the gate a denied principal reads hidden
// cardinality from Total and harvests hidden {ID, Type} pairs from
// prev/next — the exact leak shape TKT-VMD8 closes on the list path
// (CRIT finding, TKT-VMD8 review).
//
// A pile scope is the pile's readable items in the request's world, newest
// first, exactly the rows GET /_piles/{id} lists. Another user's pile is
// [piles.ErrNotFound], like a missing one.
func (a *App) resolveScope(ctx context.Context, scope ScopeDescriptor) ([]*entityPkg.Entity, error) {
	switch scope.Source {
	case "pile":
		return a.piles.scopeEntities(ctx, scope.Pile)
	case "search":
		entities, err := a.queries.executeQuery(ctx, scope.Q)
		if err != nil {
			return nil, err
		}
		if scope.Type != "" {
			filtered := entities[:0]
			for _, e := range entities {
				if e.Type == scope.Type {
					filtered = append(filtered, e)
				}
			}
			entities = filtered
		}
		return entities, nil
	default:
		return a.scopedSortedEntities(ctx, scope.Type, scope.toQuery())
	}
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
	// Set when any is present, so a half-named page scope reaches
	// resolvePageScope and is refused there instead of widening the set.
	if d.ScopePage != "" || d.ScopeTab != "" || d.Anchor != "" {
		q.Set(scopePageParam, d.ScopePage)
		q.Set(scopeTabParam, d.ScopeTab)
		q.Set(scopeAnchorParam, d.Anchor)
	}
	return q
}

// handleV1EntityPosition resolves an entity's position within a scope. It
// reproduces the scope's ordered set via resolveScope (the list pipeline for
// source=list, the search pipeline for source=search) then locates the id,
// returning {prev, next, current, total}. This supersedes the old client-side
// approach where useScopeNavigation fetched per_page=1000 and scanned the
// array — which silently truncated once the set exceeded the pagination cap
// (issue #844).
//
//	GET /api/v1/_position?id=<entityID>&scope=<urlencoded-json>
//	→ 200 {prev,next,current,total} | 400 bad scope | 404 id not in scope
func (a *App) handleV1EntityPosition(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", "GET")
		writeV1Error(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed", "")
		return
	}

	query := r.URL.Query()

	id := strings.TrimSpace(query.Get("id"))
	if id == "" {
		writeV1Error(w, r, http.StatusBadRequest, "bad_request", "id is required", "")
		return
	}

	scope, ok, reason := scopeFromParam(query.Get("scope"), a.Meta())
	if !ok {
		writeV1Error(w, r, http.StatusBadRequest, "bad_scope", "Invalid scope", reason)
		return
	}

	if pos, handled := storePosition(a, w, r, scope, id); handled {
		if pos != nil {
			writeV1JSON(w, http.StatusOK, *pos)
		}
		return
	}

	entities, err := a.resolveScope(r.Context(), scope)
	if err != nil {
		if scope.Source == "pile" {
			writePilesError(w, r, err)
			return
		}
		writeListPipelineError(w, r, err)
		return
	}

	idx := scopeIndex(scope, entities, id)
	if idx == -1 {
		writeV1Error(w, r, http.StatusNotFound, "not_in_scope", "Entity not found in scope", "")
		return
	}

	pos := v1.Position{Current: idx + 1, Total: len(entities)}
	if idx > 0 {
		pos.Prev = scopePositionRef(scope, entities[idx-1])
	}
	if idx < len(entities)-1 {
		pos.Next = scopePositionRef(scope, entities[idx+1])
	}

	writeV1JSON(w, http.StatusOK, pos)
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
	// An allowlist, not a denylist: only a list scope is a GraphQuery. A
	// search or pile scope that reached the pushdown would be read as a
	// list of an empty type.
	if scope.Source != "list" {
		return nil, false
	}
	query := scope.toQuery()
	ctx, n, err := resolveListNarrowing(r.Context(), a, scope.Type, query)
	if err != nil {
		// LOAD-BEARING: declining here is safe only because the Go path
		// (resolveScope → scopedSortedEntities) resolves the same narrowing
		// again and refuses with the same error. A scope that fails to
		// resolve must never degrade into an unscoped read; if the Go path
		// ever stops re-resolving, this must return the error instead.
		return nil, false
	}
	plan, empty, ok := n.pushdownPlan(ctx, a, scope.Type, query, 1, 1)
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
	pos := &v1.Position{Current: sp.Index, Total: sp.Total}
	if sp.Prev != nil {
		pos.Prev = &v1.PositionRef{ID: sp.Prev.ID, Type: sp.Prev.Type, Address: sp.Prev.ID}
	}
	if sp.Next != nil {
		pos.Next = &v1.PositionRef{ID: sp.Next.ID, Type: sp.Next.Type, Address: sp.Next.ID}
	}
	return pos, true
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
