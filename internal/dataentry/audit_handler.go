package dataentry

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/access"
	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/visibility"
)

const (
	auditDefaultLimit = 50
	auditMaxLimit     = 200
)

// AuditLogReader is the database audit log the audit routes read
// (TKT-X1ZFO04). sqlaudit.Log implements it. It is ungated: the routes
// decide who sees what.
type AuditLogReader interface {
	visibility.AuditEntries
	Newest(ctx context.Context, before int64, limit int) ([]audit.Entry, error)
}

// auditHandler owns /api/v1/_changes and /api/v1/_audit. Nil until
// [SetAuditLog], which is the "feature absent" signal: the fs and memory
// tiers keep a JSONL log, which cannot be paged, and offer neither route.
type auditHandler struct {
	log AuditLogReader
}

// SetAuditLog wires the database audit log into the audit routes. Without
// it both routes answer 404 and the sidebar offers neither view.
//
// A function, not an App method, because App is at its plimsoll method cap.
func SetAuditLog(a *App, log AuditLogReader) error {
	if log == nil {
		return errors.New("dataentry: SetAuditLog requires an audit log")
	}
	a.auditRead = &auditHandler{log: log}
	return nil
}

// sidebarAudit answers [v1.SidebarResponse.Audit] for the principal on ctx.
func sidebarAudit(ctx context.Context, h *auditHandler, impl acl.ACL) (changes, global bool) {
	if h == nil {
		return false, false
	}
	return true, permitsAuditRead(ctx, impl)
}

// permitsAuditRead reports whether the principal may read the global audit
// view. It is an elevated read: it shows every entity's records, deleted
// and hidden ones included, so it is refused without a policy and under
// read-only, and granted only by audit:read under a policy.
//
//archplan:guard=authz reason="checks audit:read; no policy and read-only refuse"
func permitsAuditRead(ctx context.Context, impl acl.ACL) bool {
	return access.ModeOf(impl).PermitsElevatedRead(ctx, readGateFromContext(ctx), acl.PermAuditRead)
}

// parseAuditPage reads ?before= and ?limit= and writes the 400 itself.
func parseAuditPage(w http.ResponseWriter, r *http.Request) (before int64, limit int, ok bool) {
	query := r.URL.Query()
	limit = auditDefaultLimit
	if raw := query.Get("limit"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 || n > auditMaxLimit {
			writeV1Error(w, r, http.StatusBadRequest, "invalid_limit",
				"limit must be an integer from 1 to "+strconv.Itoa(auditMaxLimit), "")
			return 0, 0, false
		}
		limit = n
	}
	if raw := query.Get("before"); raw != "" {
		n, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || n < 1 {
			writeV1Error(w, r, http.StatusBadRequest, "invalid_cursor", "before is not a value this endpoint returned", "")
			return 0, 0, false
		}
		before = n
	}
	return before, limit, true
}

// changesResponse is the body of GET /api/v1/_changes.
type changesResponse struct {
	Changes []visibility.ChangeRecord `json:"changes"`
	// NextBefore is the cursor of the next page, absent on the last.
	NextBefore int64 `json:"next_before,omitempty"`
	// WorldFaceAbsent mirrors the history timeline: the request's world
	// resolves no face, so there is no change list in it.
	WorldFaceAbsent bool `json:"world_face_absent,omitempty"`
	// Titles names the other entities the page's records name, and the users
	// who made them when a user is an entity id, by id, as the reader sees
	// them now (BUG-XR1DUQA, BUG-9WCZYOX). An id the reader cannot read, or
	// whose title is hidden, is absent.
	Titles map[string]string `json:"titles,omitempty"`
}

// handleV1Changes serves GET /api/v1/_changes/{type}/{addr}: one page of the
// audit records of one entity face, newest first (TKT-X1ZFO04).
//
// The address resolves exactly as a history read does
// ([historySubjectOr404]): a live face the principal may read, or a deleted
// face for a holder of history:read, else the uniform 404. The records are
// those of that face only, from its latest lifecycle, and each is gated by
// [visibility.EntityChanges]: a record naming an entity the principal cannot
// read is left out, and a record's text is cut to what names no entity and
// no property the principal does not see on every row of the type.
//
//archplan:auth=required
func handleV1Changes(a *App, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeV1Error(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed", "")
		return
	}
	h := a.auditRead
	if h == nil {
		writeV1Error(w, r, http.StatusNotFound, "not_found", "Not found", "")
		return
	}
	path := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/v1/_changes/"), "/")
	parts := strings.Split(path, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		writeV1Error(w, r, http.StatusBadRequest, "invalid_path", "Path must be /_changes/{type}/{id}", "")
		return
	}
	typeName := parts[0]
	ref, err := parseHistoryAddress(parts[1])
	if err != nil {
		writeV1Error(w, r, http.StatusNotFound, "not_found", entityNotFoundTitle, "")
		return
	}
	before, limit, ok := parseAuditPage(w, r)
	if !ok {
		return
	}
	subject, ok := historySubjectOr404(w, r, a.visibleReader, typeName, ref)
	if !ok {
		return
	}
	if subject.worldAbsent {
		writeV1JSON(w, http.StatusOK, changesResponse{Changes: []visibility.ChangeRecord{}, WorldFaceAbsent: true})
		return
	}
	readable := readableIDs(a.visibleReader.readableTypes)
	page, next, err := visibility.EntityChanges(r.Context(), h.log, readable, a.fieldResolver,
		visibility.ChangesQuery{
			Ref: subject.ref, Type: typeName, Before: before, Limit: limit, Live: subject.live != nil,
		})
	if err != nil {
		writeGateError(w, r, err)
		return
	}
	if page == nil {
		page = []visibility.ChangeRecord{}
	}
	writeV1JSON(w, http.StatusOK, changesResponse{
		Changes: page, NextBefore: next, Titles: changeTitles(r.Context(), a, subject.ref.ID, page),
	})
}

// changeTitles resolves, in one batch, the titles of the far ends of the
// relations page names and of the users who made its records, through the
// gated resolver in the request's world, as mentions do. A user is looked
// up as an entity id (the person a principal_property resolves to) unless
// it is a reserved or webhook identity, which the panel names by kind; a
// user that names no entity the reader may read is absent from the result.
// A title is served only whole: when the reader cannot see every property
// it is made of, the id is left out and the list shows the id (BUG-R9EHKV).
// A failed lookup is logged and serves no titles.
func changeTitles(ctx context.Context, a *App, self string, page []visibility.ChangeRecord) map[string]string {
	seen := map[string]bool{}
	var ids []string
	add := func(id string) {
		if id != "" && !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
	}
	for _, c := range page {
		if actorMayBeEntity(c.User) {
			add(c.User)
		}
		if c.Subject == nil || !c.Subject.IsRelation() {
			continue
		}
		for _, id := range []string{c.Subject.FromID, c.Subject.ToID} {
			if id != self {
				add(id)
			}
		}
	}
	if len(ids) == 0 || ctx.Err() != nil {
		return nil
	}
	served, err := a.viewReader.Resolver().ResolveIDsErr(ctx, worldFromContext(ctx).visibility(), ids)
	if err != nil {
		slog.WarnContext(ctx, "changes: title lookup failed", "ids", len(ids), "err", err)
		return nil
	}
	meta := a.Meta()
	out := make(map[string]string, len(served))
	for id, h := range served {
		if title := meta.WholeDisplayTitle(id, h.Type, h.Properties); title != id {
			out[id] = title
		}
	}
	return out
}

// idpWebhookUserPrefix is the [principal.Principal.User] prefix the
// identity provider webhook runs as: `webhook:<event>`.
const idpWebhookUserPrefix = "webhook:"

// actorMayBeEntity reports whether a record's user is worth looking up as an
// entity id: not empty, not "unknown", and not a reserved or webhook
// identity.
func actorMayBeEntity(user string) bool {
	if user == "" || user == "unknown" || principal.IsReserved(user) {
		return false
	}
	return !strings.HasPrefix(user, idpWebhookUserPrefix)
}

// readableIDs adapts a readableTypes function to [visibility.ReadableIDs].
type readableIDs func(ctx context.Context, ids []string) (map[string]string, error)

func (f readableIDs) ReadableTypes(ctx context.Context, ids []string) (map[string]string, error) {
	return f(ctx, ids)
}

// auditEntry is one record of GET /api/v1/_audit: the whole stored record
// and its seq.
type auditEntry struct {
	Seq    int64        `json:"seq"`
	Record audit.Record `json:"record"`
}

// auditResponse is the body of GET /api/v1/_audit.
type auditResponse struct {
	Records    []auditEntry `json:"records"`
	NextBefore int64        `json:"next_before,omitempty"`
}

// handleV1Audit serves GET /api/v1/_audit: one page of the whole audit log,
// newest first, to a holder of audit:read (TKT-X1ZFO04).
//
// The records are served whole and ungated, deleted entities and
// denied-write records included: audit:read is the operator's view of the
// log, and the permission is the boundary. A caller without it gets a 403
// naming the permission, since the route is configuration, not an entity.
// Every page served is itself recorded as an audit-read record, with the
// cursor and count but none of the content. That record is best-effort: it
// goes on the audit sink's queue, and the page is served even when the sink
// later drops it (which the sink logs).
//
//archplan:auth=required
func handleV1Audit(a *App, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeV1Error(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed", "")
		return
	}
	h := a.auditRead
	if h == nil {
		writeV1Error(w, r, http.StatusNotFound, "not_found", "Not found", "")
		return
	}
	ctx := r.Context()
	if !permitsAuditRead(ctx, a.acl) {
		writeV1Error(w, r, http.StatusForbidden, "permission_required", "Forbidden",
			"The audit log needs the "+acl.PermAuditRead+" permission.")
		return
	}
	before, limit, ok := parseAuditPage(w, r)
	if !ok {
		return
	}
	entries, err := h.log.Newest(ctx, before, limit+1)
	if err != nil {
		writeGateError(w, r, err)
		return
	}
	resp := auditResponse{Records: make([]auditEntry, 0, min(len(entries), limit))}
	for _, e := range entries[:min(len(entries), limit)] {
		resp.Records = append(resp.Records, auditEntry{Seq: e.Seq, Record: e.Record})
	}
	if len(entries) > limit {
		resp.NextBefore = resp.Records[len(resp.Records)-1].Seq
	}
	a.auditSink.Record(audit.Stamp(ctx, audit.Record{
		Time:       time.Now().UTC(),
		Op:         audit.OpAuditRead,
		Principal:  principal.From(ctx),
		AssertedBy: audit.AssertedByFrom(ctx),
		Summary:    fmt.Sprintf("before=%d limit=%d served=%d", before, limit, len(resp.Records)),
	}))
	writeV1JSON(w, http.StatusOK, resp)
}
