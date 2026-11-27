package dataentry

import (
	"log/slog"
	"net/http"
	"net/url"

	entityPkg "github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/visibility"
)

// redirectRetired answers a read miss on an id retired by a rename or
// conversion with a 307 to its successor, returning true when it wrote one.
// Call it only on a read miss, never for a write. The successor comes from
// [visibility.Resolver.Successor], which applies the row, face and world gates;
// an unreadable successor is no successor. A failed lookup is logged and
// treated as a miss. The redirect is `Cache-Control: no-store`. segment maps
// the successor's type to its path segment; Location is relative,
// `../{segment}/{id}`, and keeps the query but not the old face.
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
