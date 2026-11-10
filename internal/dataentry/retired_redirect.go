package dataentry

import (
	"log/slog"
	"net/http"
	"net/url"

	entityPkg "github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/visibility"
)

// redirectRetired answers a read of an id that a rename or a conversion
// retired with a 307 to the entity that took its place (TKT-2F1AUZ). It
// returns true when it wrote the redirect. On false the caller writes its
// own miss response.
//
// Call it only on a miss, so a live read pays nothing for it. It never runs
// for a write: a write sent to an old id must not land on an entity the
// caller did not name.
//
// The successor comes from [visibility.Resolver.Successor], which applies
// the row gate, the face gate and the world. A successor the caller may not
// read is no successor, so the caller then answers exactly as for an id
// that never existed. The mapping from a readable successor back to the old
// id is already served by mention resolution, so the redirect discloses
// nothing new. A store without store.RetiredIDs never redirects.
//
// A failed lookup is logged and answered as a miss: the redirect is a
// convenience, and the caller's miss answer is always a correct one.
//
// The redirect is temporary and not cached (`Cache-Control: no-store`):
// whether it applies depends on the principal and can change when the
// successor is renamed again or its read grant changes. A cached permanent
// redirect would outlive both.
//
// segment maps the successor's type to its path segment (the type name for
// `_views`, the plural for the entity routes). Location is relative to the
// request path, `../{segment}/{id}`, so a prefix in front of `/api` stays in
// place, and it keeps the request's query (the world). The old address's face
// is not carried over: the successor's type need not declare it, and the
// world still picks the face.
func redirectRetired(
	w http.ResponseWriter, r *http.Request, vis visibility.Reader, id string,
	segment func(entityType string) string,
) bool {
	ctx := r.Context()
	h, ok, err := vis.Resolver().Successor(ctx, worldFromContext(ctx).visibility(), id)
	if err != nil {
		slog.Warn("dataentry: retired-id lookup failed; answering as a miss", "id", id, "error", err)
		return false
	}
	if !ok {
		return false
	}
	seg := segment(h.Type)
	if seg == "" {
		return false
	}
	w.Header().Set("Location", retiredLocation(seg, h.ID, r.URL.RawQuery))
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusTemporaryRedirect)
	return true
}

func retiredLocation(seg, id, rawQuery string) string {
	loc := "../" + url.PathEscape(seg) + "/" + url.PathEscape(id)
	if rawQuery != "" {
		loc += "?" + rawQuery
	}
	return loc
}

// retiredRedirectID returns the bare id an address names, or "" when the
// address does not parse; a malformed address is never looked up.
func retiredRedirectID(addr string) string {
	a, err := entityPkg.ParseAddress(addr)
	if err != nil {
		return ""
	}
	return a.ID()
}
