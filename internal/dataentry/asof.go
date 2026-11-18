package dataentry

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/affordances"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// AsOfParam is the query parameter naming the instant a read answers for
// (TKT-6KLUT3U): `?as_of=2025-12-31T23:59:59Z`.
//
// It is independent of `?world=`. The world decides which face of each entity
// a read serves; the instant decides which state of the graph it reads. The
// API takes RFC 3339 with an explicit zone only. A bare date is ambiguous
// about the zone, so the SPA turns a picked date into an instant in the
// browser's zone before it sends one.
const AsOfParam = "as_of"

// asOfFromContext reports the read instant the request is bound to.
func asOfFromContext(ctx context.Context) (time.Time, bool) {
	return store.ReadInstant(ctx)
}

// readsPastInstant reports whether the request reads a past instant. Leases
// keep no history, so such a read shows none rather than today's.
func readsPastInstant(ctx context.Context) bool {
	_, ok := asOfFromContext(ctx)
	return ok
}

// attachAsOf binds `?as_of=` to the request context, refusing every
// combination this surface cannot serve.
//
// It runs inside attachWorld, so identity, the ACL request and the world are
// already bound. Identity and policy are today's: the instant changes the rows
// the gate reads, not who is asking or what the policy says (DEC-TDEF3GD).
//
// Field visibility follows the history endpoint's rules
// ([affordances.WithHistoricalSubject]): a past row's relations and the roles
// conferred on it are not today's, so only roles held globally show fields,
// and a `visible:` grant whose condition reads a relation hides its field.
// Search applies the same rules through its field scope.
//
// As-of reads are read-only. A write method is refused here, and the store
// refuses any write made under the instant as a second line.
//
// A read of the past is a history read, so it needs [acl.PermHistoryRead]. A
// past state holds content the live graph no longer has (an entity or
// relation deleted since), which the history endpoint shows only to holders
// of that permission. The 403 names it: the permission is configuration,
// not a secret.
func attachAsOf(next http.Handler, a *App) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		values, present := r.URL.Query()[AsOfParam]
		if !present || !isAPIPath(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}
		if len(values) > 1 {
			writeV1Error(w, r, http.StatusBadRequest, "as_of_duplicated",
				"duplicate as_of parameter", "give ?as_of= once")
			return
		}
		at, err := time.Parse(time.RFC3339Nano, values[0])
		if err != nil {
			writeV1Error(w, r, http.StatusBadRequest, "as_of_invalid",
				"as_of is not an RFC 3339 timestamp",
				"give an instant with a zone, for example 2025-12-31T23:59:59Z")
			return
		}
		if !readOnlyMethod(r.Method) {
			writeV1Error(w, r, http.StatusUnprocessableEntity, "as_of_read_only",
				"point-in-time reads are read-only",
				"omit ?as_of= to write the current state")
			return
		}
		if !asOfCapablePath(r.URL.Path) {
			writeV1Error(w, r, http.StatusUnprocessableEntity, "as_of_unsupported",
				"this endpoint cannot read a past state",
				"omit ?as_of= on this endpoint")
			return
		}
		if !readGateFromContext(r.Context()).HoldsPermission(r.Context(), acl.PermHistoryRead) {
			writeV1Error(w, r, http.StatusForbidden, "as_of_forbidden",
				"point-in-time reads need the "+acl.PermHistoryRead+" permission",
				"ask an administrator for "+acl.PermHistoryRead+", or omit ?as_of=")
			return
		}
		ctx, err := store.BindReadInstant(r.Context(), a.store, at)
		switch {
		case errors.Is(err, store.ErrInstantUnsupported):
			writeV1Error(w, r, http.StatusNotImplemented, "as_of_backend_unsupported",
				"this backend keeps no version history",
				"point-in-time reads need the PostgreSQL or SQLite backend")
			return
		case err != nil:
			writeV1Error(w, r, http.StatusBadRequest, "as_of_invalid", "as_of is not a usable instant", err.Error())
			return
		}
		next.ServeHTTP(w, r.WithContext(affordances.WithHistoricalSubject(ctx)))
	})
}

// asOfCapablePath reports whether path may serve a read at an instant.
//
// Like [worldCapablePath], routes join one at a time. A route joins when every
// read it makes goes through the store, because the store is what answers at
// the instant. A route that reads another source (the search index, the
// document render cache, attachments, the mention index) would answer with
// the current state, so it stays refused until that source is handled.
func asOfCapablePath(path string) bool {
	trimmed := strings.TrimPrefix(path, "/api/v1/")
	if trimmed == path {
		return false
	}
	trimmed = strings.Trim(trimmed, "/")
	if trimmed == "" {
		return false
	}
	// The entity view reads the entity, its relations and its neighbors
	// through the visibility resolver over the store.
	if isWorldCapableViewPath(trimmed) {
		return true
	}
	// A dashboard card is a list read through the list pipeline.
	if trimmed == "_dashboard/card" {
		return true
	}
	// Search answers from the store snapshot under an instant: pgstore's
	// search reads through the seam, and search.Service scans the store
	// instead of its live index.
	if trimmed == "_search" {
		return true
	}
	// A document's script and command renders read through the store, and
	// the disk cache and Lua cache are bypassed or keyed by the instant.
	if strings.HasPrefix(trimmed, "_documents/") {
		return true
	}
	// An entity's relations read the entry, its edges and its neighbors
	// through the store readers.
	if parts := strings.Split(trimmed, "/"); len(parts) == 3 && parts[0] != "" && parts[1] != "" &&
		!strings.HasPrefix(parts[0], "_") && !strings.HasPrefix(parts[1], "_") && parts[2] == "relations" {

		return true
	}
	// A gantt chart reads its nodes and edges through the store.
	if isGanttPath(trimmed) {
		return true
	}
	// `{plural}` (lists, kanban and calendar read it) and `{plural}/{id}`.
	// `{plural}/_export` and other `_` sub-routes are not admitted.
	if strings.HasPrefix(trimmed, "_") || strings.Contains(trimmed, "/_") {
		return false
	}
	return strings.Count(trimmed, "/") <= 1
}
