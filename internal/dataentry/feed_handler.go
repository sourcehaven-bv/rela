package dataentry

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/calfeed"
	entitypkg "github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
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
	cfg, found := s.Cfg.Feeds[name]
	if !found {
		writeV1Error(w, r, http.StatusNotFound, "not_found", "Feed not found", fmt.Sprintf("no feed named %q", name))
		return
	}

	// Deep links must be absolute so a calendar client can open them: a
	// relative path is useless once the .ics leaves the server. Scheme/host
	// come from the request (matching however the client reached us — loopback,
	// LAN, or a proxied hostname), mirroring appBaseURL.
	base := feedBaseURL(r)
	link := func(entityType, id string) string {
		return base + "/entity/" + entityType + "/" + id
	}
	provider, err := newDeclarativeFeed(name, cfg, s.Meta, feedEntitySource{app: a}, link)
	if err != nil {
		writeV1Error(w, r, http.StatusInternalServerError, "feed_error", "Feed misconfigured", "")
		return
	}

	feed, err := provider.renderFeed(r.Context())
	if err != nil {
		writeV1Error(w, r, http.StatusInternalServerError, "feed_error", "Failed to render feed", "")
		return
	}

	// Feed bodies embed user-authored entity content (Summary/Description are
	// read straight off entity properties in feed_provider.go), so the response
	// must never be interpreted as HTML. Two things guarantee that:
	//
	//  1. The serializers escape structurally for their own grammar, not HTML:
	//     iCalendar via calfeed.escapeText + writeLine's CRLF strip
	//     (internal/calfeed/ical.go:166, :149), JSON via encoding/json, which
	//     also escapes <, >, & to </>/& by default.
	//  2. nosniff pins the declared Content-Type, so a browser cannot
	//     MIME-sniff a text/calendar or application/json body into text/html
	//     and execute a payload the escaping above deliberately left inert
	//     (e.g. "<script>" inside an iCalendar SUMMARY, which is legal
	//     iCalendar TEXT and must stay a literal string).
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
			writeV1Error(w, r, http.StatusInternalServerError, "feed_error", "Failed to render feed", "")
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

// listType lists all entities of a type the principal may read, applying the
// ACL read-query verdict (the same DenyAll/AllowAll/Query switch the list
// endpoint uses). Fail-closed: an unresolvable verdict withholds.
func (s feedEntitySource) listType(ctx context.Context, entityType string) ([]*entitypkg.Entity, error) {
	rqr := readGateFromContext(ctx).ReadQuery(ctx, entityType)
	var out []*entitypkg.Entity
	switch {
	case rqr.DenyAll:
		return nil, nil
	case rqr.AllowAll:
		for e, err := range s.app.Services().Store.ListEntities(ctx, store.EntityQuery{Type: entityType}) {
			if err != nil {
				return nil, fmt.Errorf("feed: list %q: %w", entityType, err)
			}
			out = append(out, e)
		}
	case rqr.Query == nil:
		// Zero verdict: withhold rather than silently widening.
		return nil, nil
	default:
		for e, err := range s.app.Services().Store.GraphQuery(ctx, *rqr.Query) {
			if err != nil {
				return nil, fmt.Errorf("feed: acl-scoped list %q: %w", entityType, err)
			}
			out = append(out, e)
		}
	}
	return out, nil
}

// getEntity fetches one entity if the principal may read it (per-entity gate).
func (s feedEntitySource) getEntity(ctx context.Context, entityType, id string) (*entitypkg.Entity, bool, error) {
	return s.app.visibleReader.getVisible(ctx, entityType, id)
}
