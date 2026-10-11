package dataentry

import (
	"net/http"
	"strconv"

	v1 "github.com/Sourcehaven-BV/rela/internal/apiwire/v1"
	"github.com/Sourcehaven-BV/rela/internal/visibility"
)

// Page sizes of GET /api/v1/{plural}/{id}/mentioned-by.
const (
	mentionedByDefaultLimit = 50
	mentionedByMaxLimit     = 200
)

// handleV1MentionedBy serves the backlinks of one entity (TKT-Q6AKUN): the
// entities whose body mentions it as an `id` code span.
//
// The addressed entity is read first, so a hidden or missing one is the
// uniform 404 the entity GET gives. Each source then goes through
// visibility.Resolver.MentionedBy on the redacting view reader, the one
// forward mentions resolve through, so it is gated, ranked and redacted like
// any list row. A source's title is the display title of its served header, so a
// hidden display property falls back to the id exactly as a forward mention
// does.
func handleV1MentionedBy(a *App, w http.ResponseWriter, r *http.Request, typeName, addr string) {
	if r.Method != http.MethodGet {
		writeV1Error(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed", "")
		return
	}
	query := r.URL.Query()
	limit := mentionedByDefaultLimit
	if raw := query.Get("limit"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 || n > mentionedByMaxLimit {
			writeV1Error(w, r, http.StatusBadRequest, "invalid_limit",
				"limit must be an integer from 1 to "+strconv.Itoa(mentionedByMaxLimit), "")
			return
		}
		limit = n
	}
	after, err := visibility.ParseMentionCursor(query.Get("cursor"))
	if err != nil {
		writeV1Error(w, r, http.StatusBadRequest, "invalid_cursor", "cursor is not a value this endpoint returned", "")
		return
	}

	target, found := readAddressedOr404(w, r, a.visibleReader, typeName, addr)
	if !found {
		return
	}
	ctx := r.Context()
	page, err := a.viewReader.Resolver().MentionedBy(
		ctx, worldFromContext(ctx).visibility(), target.ID, after, limit)
	if err != nil {
		writeGateError(w, r, err)
		return
	}
	meta := a.State().Meta
	out := v1.MentionedByResponse{Items: make([]v1.MentionSource, 0, len(page.Items))}
	for _, h := range page.Items {
		out.Items = append(out.Items, v1.MentionSource{
			ID: h.ID, Face: h.Face.String(), Mention: buildMention(headerEntity(h), meta),
		})
	}
	out.NextCursor = page.Next
	out.Truncated = page.Truncated
	writeV1JSON(w, http.StatusOK, out)
}
