package dataentry

import (
	"context"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	v1 "github.com/Sourcehaven-BV/rela/internal/apiwire/v1"
	"github.com/Sourcehaven-BV/rela/internal/comments"
	entityPkg "github.com/Sourcehaven-BV/rela/internal/entity"
)

// commentCountsParam asks a collection response to carry each row's comment
// count (TKT-WA25G2). Off by default: only a kanban card that shows the count
// asks for it.
const commentCountsParam = "comment_counts"

// wantCommentCounts reports whether the request opted into comment counts.
// Anything but a parseable true is false.
func wantCommentCounts(query map[string][]string) bool {
	v, _ := strconv.ParseBool(queryGet(query, commentCountsParam))
	return v
}

// serveListRowExtras fills the per-page values a list row carries beyond its
// entity: the owner always, the comment count on request. It reports false
// after writing the error response.
func serveListRowExtras(
	w http.ResponseWriter, r *http.Request, a *App, query map[string][]string,
	rows []*entityPkg.Entity, data []v1.Entity,
) bool {
	if !serveOwners(w, r, a, rows, data) {
		return false
	}
	if wantCommentCounts(query) {
		a.comments.serveCommentCounts(r.Context(), rows, data)
	}
	return true
}

// serveCommentCounts sets the comment count on each row in data, which is
// parallel to rows. A failed count is logged and leaves every count absent:
// the count decorates a list, so it must not fail the list.
//
// The rows have already passed the read gate, so the read floor that
// [comments.Authorizer.CanRead] adds is satisfied. What remains is
// `comment:read`, and it is checked ONCE for the whole page, as a global
// grant: the per-entity form walks ancestors and role relations per row, which
// is a per-row store read on a collection path. The consequence is deliberate
// and fails closed: a principal whose `comment:read` comes only from a local
// role gets no count on list rows, though the thread itself stays readable on
// the entity.
func (h *commentsHandler) serveCommentCounts(ctx context.Context, rows []*entityPkg.Entity, data []v1.Entity) {
	if h.svc == nil || !h.readsCommentsEverywhere(ctx) {
		return
	}
	policy := h.commentPolicy()
	targets := make([]comments.Target, 0, len(rows))
	at := make([]int, 0, len(rows)) // at[j] is the row of targets[j]
	for i, e := range rows {
		if policy.Commentable(e.Type) {
			targets = append(targets, comments.Target{Type: e.Type, ID: e.ID, Face: e.Face})
			at = append(at, i)
		}
	}
	if len(targets) == 0 {
		return
	}
	counts, err := h.svc.Count(ctx, targets)
	if err != nil {
		slog.WarnContext(ctx, "dataentry: counting comments failed; _comment_count omitted", "err", err)
		return
	}
	for j, t := range targets {
		n := counts[t.Key()]
		data[at[j]].CommentCount = &n
	}
}

// readsCommentsEverywhere reports whether the principal holds `comment:read`
// as a global grant. With no declarative policy the comment routes are open,
// so this is too; with one but no request ACL on ctx it fails closed, as
// [commentPermissionGate] does.
func (h *commentsHandler) readsCommentsEverywhere(ctx context.Context) bool {
	if !h.aclPolicyActive() {
		return true
	}
	req := acl.FromContext(ctx)
	if req == nil {
		return false
	}
	return req.HoldsPermission(ctx, acl.PermCommentRead)
}
