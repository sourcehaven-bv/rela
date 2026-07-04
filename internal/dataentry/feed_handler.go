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

	provider, err := newDeclarativeFeed(name, cfg, s.Meta, feedEntitySource{app: a}, func(entityType, id string) string {
		return "/entity/" + entityType + "/" + id
	})
	if err != nil {
		writeV1Error(w, r, http.StatusInternalServerError, "feed_error", "Feed misconfigured", "")
		return
	}

	feed, err := provider.renderFeed(r.Context())
	if err != nil {
		writeV1Error(w, r, http.StatusInternalServerError, "feed_error", "Failed to render feed", "")
		return
	}

	switch format {
	case "ics":
		body := calfeed.ICal{Now: time.Now().UTC()}.RenderCollection(feed)
		w.Header().Set("Content-Type", "text/calendar; charset=utf-8")
		_, _ = w.Write(body)
	case "json":
		body, err := calfeed.RenderJSON(feed)
		if err != nil {
			writeV1Error(w, r, http.StatusInternalServerError, "feed_error", "Failed to render feed", "")
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_, _ = w.Write(body)
	}
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
