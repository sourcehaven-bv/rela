package dataentry

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	entityPkg "github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/nextaction"
	"github.com/Sourcehaven-BV/rela/internal/userstate"
)

// nextActionPage is the page a suggestion is resolved for: which surface,
// which view, and the entity it is about, if any.
//
// A client that sends no surface predates placement. Every such client
// showed suggestions only on the dashboard, so its request is read as the
// default dashboard and the operator's placement applies to it.
type nextActionPage struct {
	surface dataentryconfig.NextActionSurface
	view    string
	// contextType is the entity type the page is about, derived from config
	// (see [dataentryconfig.Config.NextActionContextType]), never from the
	// client. Empty when the page has no entity.
	contextType string
	// entityID is the BARE id of the viewed entity. A face in the address is
	// dropped on purpose: a context source reads the entity in its own
	// source_world, exactly as a query source does, so the face the reader
	// happens to be viewing must not decide what the source evaluates.
	entityID string
}

// parseNextActionPage reads the page from the query string.
//
// An unknown surface is a client bug and a 400. An unknown view is not: after
// a config reload renames a view, a loaded SPA still sends the old name, and
// an advisory surface should go quiet rather than fail. It answers ok=false,
// and the caller serves "nothing to suggest". Logged at Debug: the cause is
// expected, and the view is client text a caller could repeat at will.
//
// A malformed entity address is treated as no entity. The address is never
// echoed, and a miss of any kind must look the same as a hidden entity.
func parseNextActionPage(
	r *http.Request, cfg *dataentryconfig.Config, meta *metamodel.Metamodel,
) (nextActionPage, bool, error) {
	q := r.URL.Query()
	raw := q.Get("surface")
	if raw == "" {
		return nextActionPage{surface: dataentryconfig.SurfaceDashboard}, true, nil
	}
	surface, ok := dataentryconfig.ParseNextActionSurface(raw)
	if !ok {
		return nextActionPage{}, false, fmt.Errorf("unknown surface %q; expected one of %s",
			raw, dataentryconfig.NextActionSurfaceList())
	}
	page := nextActionPage{surface: surface, view: q.Get("view")}
	if !nextActionViewKnown(cfg, meta, page) {
		slog.Debug("dataentry: next-action request names an unknown view",
			"surface", surface, "view", page.view)
		return nextActionPage{}, false, nil
	}
	page.contextType = cfg.NextActionContextType(surface, page.view)
	if page.contextType != "" {
		if addr, err := entityPkg.ParseAddress(q.Get("entity")); err == nil {
			page.entityID = addr.ID()
		}
	}
	return page, true, nil
}

// nextActionViewKnown reports whether the page's view exists. The entity
// page's view is an entity type, which may have no configured view (the
// default view is synthesized), so it is checked against the metamodel.
func nextActionViewKnown(cfg *dataentryconfig.Config, meta *metamodel.Metamodel, page nextActionPage) bool {
	if page.surface == dataentryconfig.SurfaceEntity {
		if meta == nil {
			return false
		}
		_, ok := meta.GetEntityDef(page.view)
		return ok
	}
	return cfg.HasNextActionView(page.surface, page.view)
}

// enabled reports whether suggestions may appear on the page at all.
func (p nextActionPage) enabled(cfg *dataentryconfig.Config) bool {
	return cfg.NextActionsEnabled(p.surface, p.view)
}

// refreshType is the entity type whose changes can change this page's
// answer: the page's context type, when some source that may compete here
// is about it. Empty otherwise, so the client does not re-resolve on graph
// changes no source here can see.
func (p nextActionPage) refreshType(cfg *dataentryconfig.Config) string {
	if p.contextType == "" || p.entityID == "" {
		return ""
	}
	for id, src := range cfg.NextActions {
		if src.Context != "" && p.keeps(id, src) {
			return p.contextType
		}
	}
	return ""
}

// keeps reports whether a source may compete on this page. It runs before the
// source is queried (see [nextaction.WithSourceFilter]), so a context source
// for another type never reads, and a source placed elsewhere costs nothing.
func (p nextActionPage) keeps(_ string, src dataentryconfig.NextActionSource) bool {
	if src.Context != "" && (src.Context != p.contextType || p.entityID == "") {
		return false
	}
	return src.VisibleOnSurface(p.surface, p.view)
}

// nextActionCurrent reads the suggestion the client has on screen. Absent
// when the client sends no source.
func nextActionCurrent(r *http.Request) *userstate.Key {
	q := r.URL.Query()
	src := q.Get("current_source")
	if src == "" {
		return nil
	}
	return &userstate.Key{
		Source:   src,
		EntityID: q.Get("current_entity"),
		Variant:  q.Get("current_variant"),
	}
}

// contextCandidates yields the viewed entity as the one candidate of a
// context-aware source.
//
// The read runs in ctx, which nextActionCandidates has already rebound to the
// source's world, through the row gate. A missing entity, a hidden one and
// one of another type are all the same empty answer, so the endpoint is no
// oracle on which ids exist. A gate fault is an error, as on every read path:
// an infrastructure failure must not pass for "nothing to suggest".
//
// The entity is then field-redacted like a query candidate, with verdicts
// primed first, so the message template and the condition see only what this
// principal may see.
func (a *App) contextCandidates(
	ctx context.Context, src dataentryconfig.NextActionSource, entityID string,
) ([]nextaction.Candidate, error) {
	if entityID == "" {
		return nil, nil
	}
	e, ok, err := a.visibleReader.inWorld(ctx, src.Context, entityID)
	if err != nil {
		return nil, fmt.Errorf("next-action context %q: %w", src.Context, err)
	}
	if !ok || e.Type != src.Context {
		return nil, nil
	}
	ctx = primeVerdicts(ctx, a.fieldResolver, []*entityPkg.Entity{e})
	return []nextaction.Candidate{{Entity: a.redactedForSuggestion(ctx, e)}}, nil
}
