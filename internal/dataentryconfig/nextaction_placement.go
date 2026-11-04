package dataentryconfig

// Next-action placement: on which pages a suggestion may appear.
//
// Three keys decide it, from the widest to the narrowest:
//
//   - next_action_surfaces (top level) switches a whole surface kind on or
//     off, e.g. `kanban: false`.
//   - suggestions (on a list, kanban, calendar, gantt, view, document, page
//     or the dashboard) switches one view on or off, overriding the surface.
//   - visible_on (on a source) names the surfaces, or surface:view pairs, a
//     source may appear on. It sits on the source for the same reason
//     visible_worlds does: placing a source should mean editing that source,
//     not every view it should appear on.
//
// All three are PRESENTATION. They decide whether an already-authorized
// suggestion is worth showing on a page; the read gate has already bounded
// what any source can find. Never rely on placement to conceal anything.

import (
	"fmt"
	"sort"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// NextActionSurface is a kind of page the SPA renders. The SPA maps each
// route to one; the server uses it to apply placement and to decide which
// entity type, if any, the page is about.
type NextActionSurface string

const (
	SurfaceDashboard NextActionSurface = "dashboard"
	SurfaceList      NextActionSurface = "list"
	SurfaceKanban    NextActionSurface = "kanban"
	SurfaceCalendar  NextActionSurface = "calendar"
	SurfaceGantt     NextActionSurface = "gantt"
	// SurfaceEntity is the entity detail page. Its view is named by the
	// entity TYPE, because the detail view is chosen per type and the route
	// carries the type, not a view id.
	SurfaceEntity   NextActionSurface = "entity"
	SurfaceDocument NextActionSurface = "document"
	SurfacePage     NextActionSurface = "page"
	SurfaceForm     NextActionSurface = "form"
	SurfaceSearch   NextActionSurface = "search"
	// SurfaceOther is every page that is about the application rather than
	// the data: settings, configure, history, analyze, conflicts, apps.
	// Off by default; see [Config.NextActionsEnabled].
	SurfaceOther NextActionSurface = "other"
)

// nextActionSurfaces lists every surface kind, for validation and messages.
var nextActionSurfaces = []NextActionSurface{
	SurfaceDashboard, SurfaceList, SurfaceKanban, SurfaceCalendar, SurfaceGantt,
	SurfaceEntity, SurfaceDocument, SurfacePage, SurfaceForm, SurfaceSearch, SurfaceOther,
}

// ParseNextActionSurface returns the surface named s, or false when s names
// none.
func ParseNextActionSurface(s string) (NextActionSurface, bool) {
	for _, k := range nextActionSurfaces {
		if string(k) == s {
			return k, true
		}
	}
	return "", false
}

// NextActionsEnabled reports whether suggestions may appear on the page
// (surface, view). view is the view id; for [SurfaceEntity] it is the entity
// type; for surfaces without views it is ignored.
//
// Precedence: the view's own `suggestions:`, then `next_action_surfaces`,
// then the default. The default is on everywhere except [SurfaceOther],
// because a settings or configure screen is about the application, and a
// suggestion about the data there is noise.
//
// An unknown view counts as having no setting of its own. The handler decides
// separately whether an unknown view is worth a warning.
func (c *Config) NextActionsEnabled(surface NextActionSurface, view string) bool {
	if on, ok := c.viewSuggestions(surface, view); ok {
		return on
	}
	if on, ok := c.NextActionSurfaces[surface]; ok {
		return on
	}
	return surface != SurfaceOther
}

// viewSuggestions returns the view's own `suggestions:` setting.
func (c *Config) viewSuggestions(surface NextActionSurface, view string) (on, ok bool) {
	var set *bool
	switch surface {
	case SurfaceDashboard:
		if view != "" {
			set = c.Dashboards[view].Suggestions
		} else if c.Dashboard != nil {
			set = c.Dashboard.Suggestions
		}
	case SurfaceList:
		set = c.Lists[view].Suggestions
	case SurfaceKanban:
		set = c.Kanbans[view].Suggestions
	case SurfaceCalendar:
		set = c.Calendars[view].Suggestions
	case SurfaceGantt:
		set = c.Gantts[view].Suggestions
	case SurfaceEntity:
		if v, ok := c.ViewForEntityType(view); ok {
			set = v.Suggestions
		}
	case SurfaceDocument:
		set = c.Documents[view].Suggestions
	case SurfacePage:
		set = c.Pages[view].Suggestions
	case SurfaceForm, SurfaceSearch, SurfaceOther:
	}
	if set == nil {
		return false, false
	}
	return *set, true
}

// HasNextActionView reports whether view names a configured view of the
// surface. Surfaces without views accept only the empty view. The dashboard
// surface takes the empty view for the default dashboard and a name from
// `dashboards:` for a named one. For
// [SurfaceEntity] any type with a detail view counts, including the
// synthesized default view, so the caller checks the type against the
// metamodel.
func (c *Config) HasNextActionView(surface NextActionSurface, view string) bool {
	var ok bool
	switch surface {
	case SurfaceList:
		_, ok = c.Lists[view]
	case SurfaceKanban:
		_, ok = c.Kanbans[view]
	case SurfaceCalendar:
		_, ok = c.Calendars[view]
	case SurfaceGantt:
		_, ok = c.Gantts[view]
	case SurfaceDocument:
		_, ok = c.Documents[view]
	case SurfacePage:
		_, ok = c.Pages[view]
	case SurfaceForm:
		_, ok = c.Forms[view]
	case SurfaceDashboard:
		_, named := c.Dashboards[view]
		ok = view == "" || named
	case SurfaceEntity:
		ok = view != ""
	case SurfaceSearch, SurfaceOther:
		ok = view == ""
	}
	return ok
}

// NextActionContextType returns the entity type a page with an entity is
// about, or "" when the page has no entity context. It is decided from config,
// never taken from the client: the entity page's view IS its type, and an
// entity-anchored document, entity page or edit form declares its type.
func (c *Config) NextActionContextType(surface NextActionSurface, view string) string {
	switch surface {
	case SurfaceEntity:
		return view
	case SurfaceDocument:
		return c.Documents[view].EntityType
	case SurfacePage:
		return c.Pages[view].EntityType
	case SurfaceForm:
		return c.Forms[view].EntityType
	case SurfaceDashboard, SurfaceList, SurfaceKanban, SurfaceCalendar, SurfaceGantt,
		SurfaceSearch, SurfaceOther:
	}
	return ""
}

// ViewForEntityType returns the detail view configured for entityType.
// Validation allows at most one view per type, so the answer is unique.
func (c *Config) ViewForEntityType(entityType string) (ViewConfig, bool) {
	for _, v := range c.Views {
		if v.Entry.Type == entityType {
			return v, true
		}
	}
	return ViewConfig{}, false
}

// VisibleOnSurface reports whether a suggestion from this source may appear
// on the page (surface, view). An empty VisibleOn matches every page.
//
// An entry is either a surface kind ("list") or a surface and a view
// ("list:open-tasks"; for the entity page, "entity:<type>").
func (s NextActionSource) VisibleOnSurface(surface NextActionSurface, view string) bool {
	if len(s.VisibleOn) == 0 {
		return true
	}
	for _, entry := range s.VisibleOn {
		kind, v, scoped := strings.Cut(entry, ":")
		if kind != string(surface) {
			continue
		}
		if !scoped || v == view {
			return true
		}
	}
	return false
}

// validateNextActionPlacement checks next_action_surfaces and every source's
// visible_on. A name that matches nothing must fail the load: a misspelled
// surface would otherwise leave suggestions silently on (or a source silently
// nowhere), and nothing on screen says why.
func validateNextActionPlacement(cfg *Config, meta *metamodel.Metamodel) []string {
	var errs []string
	keys := make([]string, 0, len(cfg.NextActionSurfaces))
	for k := range cfg.NextActionSurfaces {
		keys = append(keys, string(k))
	}
	sort.Strings(keys)
	for _, k := range keys {
		if _, ok := ParseNextActionSurface(k); !ok {
			errs = append(errs, fmt.Sprintf(
				"next_action_surfaces: unknown surface %q (want one of %s)", k, NextActionSurfaceList()))
		}
	}

	for _, id := range sortedNextActionIDs(cfg.NextActions) {
		src := cfg.NextActions[id]
		where := fmt.Sprintf("next_actions[%q]", id)
		// Absent means everywhere. A written empty list would read as
		// "nowhere", a second spelling of a muted source that no user
		// can unmute, so it is refused.
		if src.VisibleOn != nil && len(src.VisibleOn) == 0 {
			errs = append(errs, where+": visible_on is empty (omit it to show the source everywhere)")
		}
		for i, entry := range src.VisibleOn {
			if msg := cfg.checkVisibleOn(entry, meta); msg != "" {
				errs = append(errs, fmt.Sprintf("%s: visible_on[%d]: %s", where, i, msg))
			}
		}
		if src.Context != "" && len(src.VisibleOn) > 0 && !cfg.anyPageAbout(src.VisibleOn, src.Context) {
			errs = append(errs, fmt.Sprintf(
				"%s: visible_on names no page about a %s, so this context source can never appear", where, src.Context))
		}
	}
	return errs
}

// anyPageAbout reports whether some page that entries name can be about an
// entity of entityType, which a context source needs to fire.
func (c *Config) anyPageAbout(entries []string, entityType string) bool {
	for _, entry := range entries {
		kind, view, scoped := strings.Cut(entry, ":")
		surface, ok := ParseNextActionSurface(kind)
		if !ok {
			continue
		}
		if scoped {
			if c.NextActionContextType(surface, view) == entityType {
				return true
			}
			continue
		}
		if c.surfaceCanBeAbout(surface, entityType) {
			return true
		}
	}
	return false
}

// surfaceCanBeAbout reports whether some view of surface is about an entity
// of entityType.
func (c *Config) surfaceCanBeAbout(surface NextActionSurface, entityType string) bool {
	switch surface {
	case SurfaceEntity:
		return true
	case SurfaceDocument:
		for _, d := range c.Documents {
			if d.EntityType == entityType {
				return true
			}
		}
	case SurfacePage:
		for _, p := range c.Pages {
			if p.EntityType == entityType {
				return true
			}
		}
	case SurfaceForm:
		for _, fm := range c.Forms {
			if fm.EntityType == entityType {
				return true
			}
		}
	case SurfaceDashboard, SurfaceList, SurfaceKanban, SurfaceCalendar, SurfaceGantt,
		SurfaceSearch, SurfaceOther:
	}
	return false
}

// checkVisibleOn returns why entry names no page, or "" when it is fine.
// A view of the entity surface is an entity type, checked against meta when
// one is available.
func (c *Config) checkVisibleOn(entry string, meta *metamodel.Metamodel) string {
	kind, view, scoped := strings.Cut(entry, ":")
	surface, ok := ParseNextActionSurface(kind)
	if !ok {
		return fmt.Sprintf("unknown surface %q (want one of %s)", kind, NextActionSurfaceList())
	}
	if !scoped {
		return ""
	}
	if surface == SurfaceEntity && meta != nil && view != "" {
		if _, ok := meta.GetEntityDef(view); !ok {
			return fmt.Sprintf("%q names no entity type", view)
		}
		return ""
	}
	if view == "" || !c.HasNextActionView(surface, view) {
		return fmt.Sprintf("%q names no %s view", view, surface)
	}
	return ""
}

// NextActionSurfaceList names every surface, comma-separated, for error
// messages.
func NextActionSurfaceList() string {
	names := make([]string, len(nextActionSurfaces))
	for i, s := range nextActionSurfaces {
		names[i] = string(s)
	}
	return strings.Join(names, ", ")
}
