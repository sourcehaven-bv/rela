package dataentry

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"slices"
	"strings"
	"unicode"

	v1 "github.com/Sourcehaven-BV/rela/internal/apiwire/v1"
	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	entityPkg "github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/visibility"
)

// handleV1NavItems serves GET /api/v1/_nav_items: the entries of each
// navigation group that declares `items_from:`, for this principal, keyed by
// the group's items key. With `spaces:` configured, `?space=<id>` limits it
// to the groups of the space the sidebar resolves for the same id.
//
// An entry is a row of the group's list, labeled with the row's title. A
// title is entity content, so it must never reach /api/v1/_sidebar, which is
// the same for every principal (docs/acl-security.md, "Sidebar menu structure
// is principal-independent"). It is also kept apart from /api/v1/_nav_status,
// whose labels must never carry entity content.
//
// A package function taking the App rather than a method: App is at its
// plimsoll load line, and this needs only what it names.
func handleV1NavItems(a *App, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeV1Error(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed", "")
		return
	}
	ctx := r.Context()
	s := a.State()
	aclImpl := a.views.currentACL()

	items := make(map[string]v1.NavItemList)
	for _, ni := range sidebarNavItemsEntries(ctx, aclImpl, s.Cfg, r.URL.Query().Get("space")) {
		list, err := navItemsFor(ctx, a, s, *ni.Entry.ItemsFrom)
		if err != nil {
			// One group that cannot be read must not blank the others, and an
			// absent group is the honest answer for it.
			slog.WarnContext(ctx, "nav items: group skipped",
				"group", ni.Entry.Group, "list", ni.Entry.ItemsFrom.List, "error", err)
			continue
		}
		items[ni.Key] = list
	}

	// Per principal; writeV1JSON already marks every v1 response no-store,
	// which is what keeps a shared cache from holding one principal's titles.
	writeV1JSON(w, http.StatusOK, v1.NavItemsResponse{Items: items})
}

// navItemsFor returns the entries of one generated group: the first rows of
// its list, as the list shows them to this principal.
//
// The rows come from listPage, the list endpoint's own pipeline, so they are
// exactly the list's rows: row-gated, scoped, filtered, conditioned and
// ordered, and paged in the store when the list allows it. Each row is
// redacted once before its title is taken, so a hidden display property
// cannot reach the label. Truncated compares the total the list counted
// after all of that with the limit, so it says nothing about hidden rows.
func navItemsFor(
	ctx context.Context, a *App, s *Schema, from dataentryconfig.NavItemsFrom,
) (v1.NavItemList, error) {
	listCfg, found := s.Cfg.Lists[from.List]
	if !found {
		return v1.NavItemList{}, fmt.Errorf("unknown list %q", from.List)
	}
	limit := from.EffectiveLimit()
	rows, total, _, err := a.listPage(ctx, listCfg.EntityType, navItemsListQuery(from.List, listCfg), 1, limit)
	if err != nil {
		return v1.NavItemList{}, err
	}

	red := appRedactor(a)
	redacted := make([]*entityPkg.Entity, 0, len(rows))
	for _, e := range rows {
		redacted = append(redacted, visibility.Redact(ctx, red, e))
	}
	initials, err := navItemInitials(ctx, a, s, from.Initial, listCfg.EntityType, redacted)
	if err != nil {
		return v1.NavItemList{}, err
	}

	entries := make([]v1.NavItem, 0, len(redacted))
	for _, e := range redacted {
		entries = append(entries, v1.NavItem{
			ID:      e.ID,
			Type:    e.Type,
			Label:   s.Meta.DisplayTitle(e.ID, e.Type, e.Properties),
			Initial: initials[e.ID],
		})
	}
	return v1.NavItemList{Entries: entries, Truncated: total > limit}, nil
}

// navItemsListQuery builds the list request the SPA sends for a list's own
// page (listBaseParams in listParams.ts): the static filters, the list id
// that applies its `condition:`, its sort (the group property first when it
// is grouped) and its query scope.
func navItemsListQuery(listID string, listCfg dataentryconfig.List) map[string][]string {
	query := navStatusListQuery(listCfg)
	query[listIDParam] = []string{listID}
	var sort []string
	if listCfg.GroupBy != nil && listCfg.GroupBy.Property != "" {
		sort = append(sort, listCfg.GroupBy.Property)
	}
	for _, spec := range listCfg.Sort {
		if listCfg.GroupBy != nil && spec.Property == listCfg.GroupBy.Property {
			continue
		}
		if spec.IsDescending() {
			sort = append(sort, "-"+spec.Property)
		} else {
			sort = append(sort, spec.Property)
		}
	}
	if len(sort) > 0 {
		query["sort"] = []string{strings.Join(sort, ",")}
	}
	if listCfg.QueryScope != "" {
		query[QueryScopeParam] = []string{listCfg.QueryScope}
	}
	return query
}

// navItemInitials returns the letter badge of each row that has one, keyed by
// row id. rows are already redacted.
//
// A property initial is the first letter of the row's own property, so a
// redacted property gives none. A relation initial is the first letter of the
// title of a related entity, such as the row's owner (see
// navItemRelationInitials).
func navItemInitials(
	ctx context.Context, a *App, s *Schema, in *dataentryconfig.NavItemsInitial,
	rowType string, rows []*entityPkg.Entity,
) (map[string]string, error) {
	out := make(map[string]string, len(rows))
	if in == nil || len(rows) == 0 {
		return out, nil
	}
	if in.Relation != "" {
		return navItemRelationInitials(ctx, a, s, in, rowType, rows)
	}
	for _, e := range rows {
		if v, ok := e.Properties[in.Property]; ok && v != nil {
			if initial := firstLetter(fmt.Sprint(v)); initial != "" {
				out[e.ID] = initial
			}
		}
	}
	return out, nil
}

// navItemRelationInitials returns the relation initial of each row that has
// one. The neighbors are read in one batch for all rows: one relation query,
// one header read, one gate probe per neighbor type. A neighbor the principal
// may not read gives no initial, and neither does one whose title is hidden.
// When a row has several readable neighbors, the one with the lowest id wins,
// so the badge does not change between requests.
func navItemRelationInitials(
	ctx context.Context, a *App, s *Schema, in *dataentryconfig.NavItemsInitial,
	rowType string, rows []*entityPkg.Entity,
) (map[string]string, error) {
	neighborsOf, neighborIDs, err := navItemNeighbors(ctx, a, s, in, rowType, rows)
	if err != nil || len(neighborIDs) == 0 {
		return map[string]string{}, err
	}
	neighborInitial, err := navItemNeighborInitials(ctx, a, s, neighborIDs)
	if err != nil {
		return nil, err
	}
	out := make(map[string]string, len(rows))
	for rowID, neighbors := range neighborsOf {
		slices.Sort(neighbors)
		for _, n := range neighbors {
			if initial := neighborInitial[n]; initial != "" {
				out[rowID] = initial
				break
			}
		}
	}
	return out, nil
}

// navItemNeighbors reads the initial's relation for all rows in one query. It
// returns each row's neighbor ids and the distinct neighbor ids.
func navItemNeighbors(
	ctx context.Context, a *App, s *Schema, in *dataentryconfig.NavItemsInitial,
	rowType string, rows []*entityPkg.Entity,
) (neighborsOf map[string][]string, neighborIDs []string, err error) {
	ids := make([]string, 0, len(rows))
	for _, e := range rows {
		ids = append(ids, e.ID)
	}
	dir := in.ResolvedDirection(rowType, s.Meta)
	neighborsOf = make(map[string][]string, len(rows))
	seen := make(map[string]bool)
	q := store.RelationQuery{EntityIDs: ids, Type: in.Relation, Direction: relationDirection(dir)}
	for r, err := range a.Services().Store.ListRelations(ctx, q) {
		if err != nil {
			return nil, nil, err
		}
		rowID, neighborID := r.From, r.To
		if dir.IsIncoming() {
			rowID, neighborID = r.To, r.From
		}
		neighborsOf[rowID] = append(neighborsOf[rowID], neighborID)
		if !seen[neighborID] {
			seen[neighborID] = true
			neighborIDs = append(neighborIDs, neighborID)
		}
	}
	return neighborsOf, neighborIDs, nil
}

// navItemNeighborInitials reads the neighbors' headers in one batch and
// returns the initial of each one the principal may read, taken from its
// redacted title.
func navItemNeighborInitials(ctx context.Context, a *App, s *Schema, ids []string) (map[string]string, error) {
	// One gated batch read, each id resolved to the face the request's
	// world selects.
	headers, err := a.visibleReader.resolver.ResolveIDsErr(ctx, worldFromContext(ctx).visibility(), ids)
	if err != nil {
		return nil, err
	}
	red := appRedactor(a)
	out := make(map[string]string, len(headers))
	for _, h := range headers {
		rh := visibility.RedactHeader(ctx, red, h)
		// A title that fell back to the id means the display property is
		// unset or hidden, and a letter of the id says nothing about who
		// the neighbor is.
		if title := s.Meta.DisplayTitle(rh.ID, rh.Type, rh.Properties); title != rh.ID {
			out[h.ID] = firstLetter(title)
		}
	}
	return out, nil
}

// firstLetter returns the first letter or digit of s, upper-cased, or "".
func firstLetter(s string) string {
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return string(unicode.ToUpper(r))
		}
	}
	return ""
}
