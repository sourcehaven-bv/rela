package dataentry

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/calfeed"
	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	entitypkg "github.com/Sourcehaven-BV/rela/internal/entity"
)

// feedPathPrefix is the route prefix for calendar feeds.
const feedPathPrefix = "/api/v1/_feeds/"

// handleV1Feed serves a declarative calendar feed as iCalendar or JSON at
// GET /api/v1/_feeds/<name>.<ext>. It runs on the inner /api/ mux, so the ACL
// read gate and the same-origin/CSRF chain already apply; the feed path is
// additionally CSRF-exempt (see nonBrowserExemptPrefixes) because a calendar
// poller is a non-browser client.
//
// Phase 1 is loopback-trust: the request's principal is whatever the
// --principal-header chain resolved (default on a bare loopback request), and
// entities are ACL-scoped to that principal. Networked/token auth is a later
// addition on the same principal-resolution seam.
//
//archplan:auth=required
func (a *App) handleV1Feed(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeV1Error(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed", "")
		return
	}

	name, format, ok := parseFeedPath(strings.TrimPrefix(r.URL.Path, feedPathPrefix))
	if !ok {
		writeV1Error(w, r, http.StatusNotFound, "not_found", "Feed not found", "")
		return
	}

	s := a.State()
	feedID, cfg, found := configuredFeed(s.Cfg.Feeds, name)
	if !found {
		writeV1Error(w, r, http.StatusNotFound, "not_found", "Feed not found", fmt.Sprintf("no feed named %q", name))
		return
	}

	base := feedBaseURL(r)
	link := func(entityType, id string) string {
		return base + "/entity/" + entityType + "/" + id
	}
	provider, err := newDeclarativeFeed(feedID, cfg, s.Meta, feedEntitySource{app: a}, link, appRedactor(a))
	if err != nil {
		writeInternalError(w, r, "feed_error", "Feed misconfigured", err)
		return
	}

	feed, err := provider.renderFeed(r.Context())
	if err != nil {
		writeInternalError(w, r, "feed_error", "Failed to render feed", err)
		return
	}

	w.Header().Set("X-Content-Type-Options", "nosniff")

	switch format {
	case "ics":
		body := calfeed.ICal{Now: time.Now().UTC()}.RenderCollection(feed)
		w.Header().Set("Content-Type", "text/calendar; charset=utf-8")
		// #nosec G705 -- not an HTML sink: iCalendar TEXT escaped by
		// calfeed.escapeText (internal/calfeed/ical.go:166) and served as
		// text/calendar under the nosniff header set above.
		_, _ = w.Write(body)
	case "json":
		body, err := calfeed.RenderJSON(feed)
		if err != nil {
			writeInternalError(w, r, "feed_error", "Failed to render feed", err)
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		// #nosec G705 -- not an HTML sink: encoding/json escapes <, >, & to
		// </>/& (internal/calfeed/json.go RenderJSON) and the
		// body is served as application/json under the nosniff header above.
		_, _ = w.Write(body)
	}
}

// feedBaseURL returns the absolute "scheme://host" prefix for deep links,
// derived from the request so links resolve however the client reached the
// server. Scheme follows TLS / an X-Forwarded-Proto hint. When the Host is
// empty or unsafe it returns "" — links then fall back to relative, which is
// no worse than before.
func feedBaseURL(r *http.Request) string {
	if r.Host == "" || hostUnsafeForCSP.MatchString(r.Host) {
		return ""
	}
	scheme := "http"
	if r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https") {
		scheme = "https"
	}
	return scheme + "://" + r.Host
}

// parseFeedPath splits "<name>.<ext>" into the feed name and a supported
// extension. ok is false for a missing/unsupported extension or an empty name.
func parseFeedPath(tail string) (name, format string, ok bool) {
	dot := strings.LastIndexByte(tail, '.')
	if dot <= 0 {
		return "", "", false
	}
	name, ext := tail[:dot], tail[dot+1:]
	if strings.Contains(name, "/") {
		return "", "", false
	}
	switch ext {
	case "ics", "json":
		return name, ext, true
	default:
		return "", "", false
	}
}

// feedEntitySource is the production entitySource: it lists and fetches entities
// through the ACL read gate on the request context, so a feed only ever exposes
// entities the request's principal may read. It is a thin adapter over App
// (deliberately not an App method, to keep App's surface from growing).
type feedEntitySource struct {
	app *App
}

// listType lists all entities of a type the principal may read.
//
// Reads through scopedQuery and scanHeaders (scopedread.go), which own the
// verdict switch and every narrowing. This used to restate the switch by
// hand, which is how a world or face narrowing could reach the list endpoint
// and not the feed. Fail-closed: an unresolvable verdict withholds. The scan
// grows with the type; TKT-SMUO8CS pushes the feed's `where:` into the store.
func (s feedEntitySource) listType(ctx context.Context, entityType string) ([]*entitypkg.Entity, error) {
	rqr := readGateFromContext(ctx).ReadQuery(ctx, entityType)
	req := scopeRequest{Type: entityType, Faces: rqr.Faces}
	q, errClass, withheld, err := scopedQuery(ctx, rqr, req)
	if withheld || err != nil {
		return nil, wrapFeedList(entityType, err)
	}
	headers, err := scanHeaders(ctx, s.app.Services(), q, errClass, req)
	if err != nil {
		return nil, wrapFeedList(entityType, err)
	}
	return headerEntities(headers), nil
}

// listByIDs is listType narrowed to the named ids: the same verdict, world
// and faces, at a cost that follows len(ids) rather than the type. An id the
// principal may not read is absent from the result.
func (s feedEntitySource) listByIDs(ctx context.Context, entityType string, ids []string) ([]*entitypkg.Entity, error) {
	rqr := readGateFromContext(ctx).ReadQuery(ctx, entityType)
	entities, err := scopedEntities(ctx, s.app.Services(), rqr, scopeRequest{
		Type: entityType, Faces: rqr.Faces, ByIDs: true, IDs: ids,
	})
	if err != nil {
		return nil, wrapFeedList(entityType, err)
	}
	return entities, nil
}

// wrapFeedList names the feed source type in a read error; nil stays nil.
func wrapFeedList(entityType string, err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("feed: list %q: %w", entityType, err)
}

// getEntity fetches one entity if the principal may read it (per-entity gate).
func (s feedEntitySource) getEntity(ctx context.Context, entityType, addr string) (*entitypkg.Entity, bool, error) {
	return s.app.visibleReader.address(ctx, entityType, addr)
}

// configuredFeed returns the configured feed named name, with its key read
// from the config map rather than from the request. The key becomes the
// default calendar name in the feed body, so the body carries only
// operator-authored text, never the request path.
func configuredFeed(feeds map[string]dataentryconfig.Feed, name string) (string, dataentryconfig.Feed, bool) {
	for key, cfg := range feeds {
		if key == name {
			return key, cfg, true
		}
	}
	return "", dataentryconfig.Feed{}, false
}
