package dataentry

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"

	entityPkg "github.com/Sourcehaven-BV/rela/internal/entity"
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
	Source  string            `json:"source"`            // "list" | "search"
	Type    string            `json:"type"`              // entity type name (singular)
	Filters map[string]string `json:"filters,omitempty"` // filter[...] bracket keys → value
	Sort    string            `json:"sort,omitempty"`    // "-created,title" form
	Q       string            `json:"q,omitempty"`       // free-text query
}

// knownScopeSources gates Source. Extending scope to a new origin is a
// deliberate change: add the source here and wire whatever produces it. An
// unknown source is rejected rather than silently treated as a plain list, so
// a typo in the SPA surfaces as a 400 instead of a wrong result set.
var knownScopeSources = map[string]struct{}{
	"list":   {},
	"search": {},
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
	case "list":
		if d.Type == "" {
			return ScopeDescriptor{}, false, "scope type is required"
		}
	case "search":
		if strings.TrimSpace(d.Q) == "" {
			return ScopeDescriptor{}, false, "scope q is required for search"
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
func (a *App) resolveScope(ctx context.Context, scope ScopeDescriptor) ([]*entityPkg.Entity, error) {
	switch scope.Source {
	case "search":
		entities := a.executeQuery(ctx, scope.Q)
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
	return q
}

// V1Position is the scope-navigator payload: the four scalars the SPA needs to
// render prev/next and the "[current/total]" counter, with no entity bodies
// shipped. current is 1-based; prev/next are nil at the ends of the set.
type V1Position struct {
	Prev    *string `json:"prev"`
	Next    *string `json:"next"`
	Current int     `json:"current"`
	Total   int     `json:"total"`
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

	entities, err := a.resolveScope(r.Context(), scope)
	if err != nil {
		writeV1Error(w, r, http.StatusInternalServerError, "search_failed", "Free-text search failed", err.Error())
		return
	}

	idx := -1
	for i, e := range entities {
		if e.ID == id {
			idx = i
			break
		}
	}
	if idx == -1 {
		writeV1Error(w, r, http.StatusNotFound, "not_in_scope", "Entity not found in scope", "")
		return
	}

	pos := V1Position{Current: idx + 1, Total: len(entities)}
	if idx > 0 {
		pos.Prev = &entities[idx-1].ID
	}
	if idx < len(entities)-1 {
		pos.Next = &entities[idx+1].ID
	}

	writeV1JSON(w, http.StatusOK, pos)
}
