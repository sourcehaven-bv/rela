package dataentry

import (
	"context"
	"crypto/sha256"
	"encoding/base32"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/emersion/go-webdav"
	"github.com/emersion/go-webdav/caldav"

	"github.com/Sourcehaven-BV/rela/internal/calfeed"
	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	entitypkg "github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/filter"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/visibility"
)

// caldavPathPrefix is the route prefix for the CalDAV server.
//
// Mounted under /api/ deliberately: attachACLRequest and requireVerifiedJWT are
// both gated on isAPIPath, so a sibling prefix would get NO acl.Request, NO
// read gate (readGateFromContext returns a permit-all nopReadGate) and NO JWT
// gate — the very gate that carries the upstream identity assertion. This is a
// security requirement, not a URL preference.
const caldavPathPrefix = "/api/v1/_caldav/"

// The URL layout is FIXED by go-webdav, which derives a path's resource kind
// from its depth below Prefix: root / principal / home-set / calendar / object.
// So the segments below are not free choices — collapsing two of them (an
// earlier attempt made the principal and the home set the same path) makes the
// library classify a home-set PROPFIND as a principal and return an empty
// multistatus.
//
// Collection URLs use the CONFIG KEY as the final segment, never a generated
// id: a changing href makes every client re-add the whole list as new, and
// Thunderbird does not auto-discover collections at all (each calendar is added
// by pasting its URL), so the path must stay stable and human-typable.
const (
	caldavPrincipalSegment = "principal"
	caldavHomeSegment      = "calendars"
)

// caldavBackend implements caldav.Backend over rela's entity store.
//
// It is the ONLY place go-webdav's types appear. calfeed stays the domain
// model and this adapter converts at the boundary, so *ical.Calendar never
// escapes into the rest of the codebase (the "don't leak parsing types" rule).
type caldavBackend struct {
	app *App
	// baseURL is the scheme://host the CLIENT used to reach us, captured per
	// request. Deep links must be absolute: a relative path is useless once the
	// iCalendar leaves the server, and go-webdav's Backend interface carries no
	// *http.Request, so the value has to ride on the backend itself. Empty is
	// tolerated (links degrade to relative) rather than fatal.
	baseURL string
	// redactor overrides the app's `visible:` field redactor. Nil in production
	// (fieldRedactor falls back to the app's); set only by tests that pin the
	// redaction WIRING, which is the part a regression would silently remove.
	redactor visibility.FieldRedactor
}

// principalPath is the WebDAV principal for the acting caller.
//
// A single fixed principal path is correct here even though rela is
// multi-user: the ACL scopes every read to the request's principal, so two
// users hitting the same path see different collections. Encoding the user in
// the path would leak the identity into a URL the client stores, without
// adding a boundary.
func (b *caldavBackend) CurrentUserPrincipal(context.Context) (string, error) {
	return caldavPathPrefix + caldavPrincipalSegment + "/", nil
}

func (b *caldavBackend) CalendarHomeSetPath(context.Context) (string, error) {
	return caldavPathPrefix + caldavPrincipalSegment + "/" + caldavHomeSegment + "/", nil
}

// ListCalendars enumerates the configured collections.
//
// This is what makes one account URL yield several lists: the client issues
// PROPFIND Depth:1 over the home set and discovers every collection, so an
// operator declares a collection per entity type and the user configures the
// account once.
// Takes ctx (it previously discarded one) because DYNAMIC patterns expand from
// the graph: the set of collections is now per-principal, since each expands to
// one collection per driver entity THIS caller may read.
func (b *caldavBackend) ListCalendars(ctx context.Context) ([]caldav.Calendar, error) {
	cfg := b.app.State().Cfg
	names := make([]string, 0, len(cfg.CalDAV.Static))
	for name := range cfg.CalDAV.Static {
		names = append(names, name)
	}
	sort.Strings(names) // stable order; map iteration is randomized

	out := make([]caldav.Calendar, 0, len(names))
	for _, name := range names {
		out = append(out, b.calendarFor(name, cfg.CalDAV.Static[name]))
	}

	patterns := make([]string, 0, len(cfg.CalDAV.Dynamic))
	for name := range cfg.CalDAV.Dynamic {
		patterns = append(patterns, name)
	}
	sort.Strings(patterns)
	for _, pattern := range patterns {
		expanded, err := b.expandPattern(ctx, pattern, cfg.CalDAV.Dynamic[pattern])
		if err != nil {
			return nil, err
		}
		out = append(out, expanded...)
	}
	return out, nil
}

// expandPattern lists one collection per driver entity the caller may read.
//
// Drivers come from feedEntitySource.listType, the same ACL-gated source every
// other read uses, so a driver the principal cannot see yields no collection —
// absent from enumeration rather than present-and-forbidden, which would be an
// existence oracle for the driver id sitting in the URL.
//
// The display name is the driver's title, resolved through the metamodel, so
// renaming a project renames its list. The URL segment is built from the driver
// ID and therefore does NOT move — a changed collection href makes a client
// re-add the whole list as new.
func (b *caldavBackend) expandPattern(
	ctx context.Context, pattern string, dyn dataentryconfig.CalDAVDynamicCollection,
) ([]caldav.Calendar, error) {
	src := feedEntitySource{app: b.app}
	drivers, err := src.listType(ctx, dyn.DriverType)
	if err != nil {
		return nil, err
	}
	meta := b.app.State().Meta
	out := make([]caldav.Calendar, 0, len(drivers))
	for _, d := range drivers {
		cfg := dyn.CalDAVCollection
		// meta.name on a pattern would name every expansion identically, so the
		// driver's display title is the label. Falls back to the id.
		cfg.Meta.Name = meta.DisplayTitle(d.ID, d.Type, d.Properties)
		if cfg.Meta.Name == "" {
			cfg.Meta.Name = d.ID
		}
		out = append(out, b.calendarFor(dynamicName(pattern, d.ID), cfg))
	}
	return out, nil
}

func (b *caldavBackend) GetCalendar(ctx context.Context, p string) (*caldav.Calendar, error) {
	name, _, ok := b.splitPath(p)
	if !ok {
		return nil, webdav.NewHTTPError(http.StatusNotFound, errors.New("caldav: unknown collection"))
	}
	if cfg, found := b.app.State().Cfg.CalDAV.Static[name]; found {
		cal := b.calendarFor(name, cfg)
		return &cal, nil
	}
	dyn, driverID, resolved := b.resolveDynamic(ctx, name)
	if !resolved {
		if driverID != "" {
			// A well-formed dynamic name whose driver is absent OR unreadable —
			// the same answer for both, so the URL cannot probe existence.
			return nil, notFoundHere()
		}
		return nil, webdav.NewHTTPError(http.StatusNotFound, errors.New("caldav: unknown collection"))
	}
	cfg := dyn.CalDAVCollection
	src := feedEntitySource{app: b.app}
	if d, visible, err := src.getEntity(ctx, dyn.DriverType, driverID); err == nil && visible {
		cfg.Meta.Name = b.app.State().Meta.DisplayTitle(d.ID, d.Type, d.Properties)
	}
	if cfg.Meta.Name == "" {
		cfg.Meta.Name = driverID
	}
	cal := b.calendarFor(name, cfg)
	return &cal, nil
}

// calendarFor builds the advertised metadata for one collection.
func (b *caldavBackend) calendarFor(name string, cfg dataentryconfig.CalDAVCollection) caldav.Calendar {
	display := cfg.Meta.Name
	if display == "" {
		display = name
	}
	// A collection advertises exactly ONE component. Apple's clients segregate
	// by component set — Reminders binds only to a VTODO collection and
	// Calendar.app creates its own separate VEVENT one — so a mixed collection
	// is invisible to one of them.
	component := "VTODO"
	if cfg.ComponentOrDefault() == dataentryconfig.CalDAVComponentEvent {
		component = "VEVENT"
	}
	return caldav.Calendar{
		Path:                  b.calendarPath(name),
		Name:                  display,
		Description:           cfg.Meta.Description,
		SupportedComponentSet: []string{component},
	}
}

func (b *caldavBackend) calendarPath(name string) string {
	return caldavPathPrefix + caldavPrincipalSegment + "/" + caldavHomeSegment + "/" + name + "/"
}

// splitPath decomposes a request path into the collection name and, when the
// path addresses a resource rather than the collection itself, its href.
func (b *caldavBackend) splitPath(p string) (name, href string, ok bool) {
	// Strip the prefix BEFORE cleaning. path.Clean resolves ".." first, so
	// cleaning the whole path would let ".../tasks/../other/a.ics" address
	// collection "other" while claiming to be scoped to "tasks" — defeating any
	// reasoning that a request stays within the collection it names.
	rest, found := strings.CutPrefix(p, caldavPathPrefix+caldavPrincipalSegment+"/"+caldavHomeSegment+"/")
	if !found {
		return "", "", false
	}
	rest = strings.Trim(rest, "/")
	if rest == "" {
		return "", "", false
	}
	name, href, hasHref := strings.Cut(rest, "/")
	// A collection name is a config key and an href is one path segment, so
	// neither may contain a separator or a traversal element. Rejecting an href
	// with a slash also keeps the alias key unambiguous — otherwise the same
	// resource is addressable under two keys.
	if name == "" || name == "." || name == ".." || strings.Contains(href, "/") {
		return "", "", false
	}
	if href == "." || href == ".." {
		return "", "", false
	}
	if !hasHref {
		return name, "", true
	}
	return name, href, true
}

// splitDynamicName decomposes a collection segment into its pattern key and
// driver entity id: "project_tasks--PROJ-1" → ("project_tasks", "PROJ-1", true).
//
// ok=false means the segment is not a dynamic collection name — a static key, or
// a pattern key with no driver, or a driver id that is not a legal entity id.
// The caller then treats it as static, so an unknown segment falls through to
// the same 404 it always got.
//
// # Why the id is validated here
//
// The driver id lands in a store lookup and in the alias key. entity.ValidateID
// pins ids to ^[A-Za-z0-9][A-Za-z0-9_-]*$ — no slash, no dot, no percent — so
// validating here means a hostile segment is rejected BEFORE it reaches either.
// splitPath has already rejected path separators and traversal elements; this is
// the narrower check that the remainder is an id at all.
//
// SplitN with n=2 on the FIRST separator, not the last: a pattern key cannot
// contain "--" (config validation rejects it) and an id cannot either, so the
// first occurrence is the only boundary. Splitting on the last would let a
// crafted segment smuggle a different pattern name past the lookup.
func splitDynamicName(segment string) (pattern, driverID string, ok bool) {
	parts := strings.SplitN(segment, feedUIDSep, 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", false
	}
	if err := entitypkg.ValidateID(parts[1]); err != nil {
		return "", "", false
	}
	return parts[0], parts[1], true
}

// dynamicName builds the URL segment for one expanded collection. Inverse of
// [splitDynamicName].
func dynamicName(pattern, driverID string) string {
	return pattern + feedUIDSep + driverID
}

// mapperFor resolves the mapper for a collection, or a 404.
// Takes ctx because a DYNAMIC collection's existence is a graph question: the
// driver entity must exist AND be readable by this principal. A hidden driver
// yields the same 404 as an unknown one — the driver id is in the URL, and
// entity existence is the thing the read gate protects (RR-NGMI).
func (b *caldavBackend) mapperFor(
	ctx context.Context, name string,
) (*caldavMapper, dataentryconfig.CalDAVCollection, error) {
	s := b.app.State()
	link := func(entityType, id string) string {
		return b.baseURL + "/entity/" + entityType + "/" + id
	}
	if cfg, ok := s.Cfg.CalDAV.Static[name]; ok {
		return newCalDAVMapper(name, cfg, s.Meta, link), cfg, nil
	}
	if dyn, driverID, ok := b.resolveDynamic(ctx, name); ok {
		return newCalDAVMapper(name, dyn.CalDAVCollection, s.Meta, link), dyn.CalDAVCollection, nil
	} else if driverID != "" {
		// A well-formed dynamic name whose driver is absent or unreadable.
		return nil, dataentryconfig.CalDAVCollection{}, notFoundHere()
	}
	return nil, dataentryconfig.CalDAVCollection{},
		webdav.NewHTTPError(http.StatusNotFound, errors.New("caldav: unknown collection"))
}

// resolveDynamic resolves a composite segment to its pattern config, if the
// segment names a configured pattern whose driver entity this principal may
// read.
//
// The second return is the parsed driver id, non-empty whenever the segment WAS
// a well-formed dynamic name — so the caller can distinguish "not a dynamic
// name at all" from "a dynamic name that did not resolve", and answer the
// latter with the uniform not-found rather than a different status.
func (b *caldavBackend) resolveDynamic(
	ctx context.Context, name string,
) (cfg dataentryconfig.CalDAVDynamicCollection, driverID string, ok bool) {
	pattern, id, split := splitDynamicName(name)
	if !split {
		return cfg, "", false
	}
	dyn, found := b.app.State().Cfg.CalDAV.Dynamic[pattern]
	if !found {
		return cfg, "", false
	}
	// The driver must be readable BY THIS PRINCIPAL. getEntity returns
	// (nil,false,nil) for both absent and denied, deliberately
	// indistinguishable, which is exactly the answer wanted here.
	src := feedEntitySource{app: b.app}
	if _, visible, err := src.getEntity(ctx, dyn.DriverType, id); err != nil || !visible {
		return cfg, id, false
	}
	return dyn, id, true
}

// dynamicMembers returns the ids belonging to a dynamic collection, via ONE
// relation traversal from the driver entity.
//
// scoped=false means the collection is static and every entity of the type is a
// candidate — the caller then applies no membership filter at all, rather than
// treating an empty set as "nothing matches".
//
// # One traversal from the DRIVER, not one per member
//
// The query is anchored on the driver entity, so it costs a single
// RelationQuery regardless of how many entities of the member type exist.
// Walking members instead ("for each task, does it link to this project?")
// would be O(members) traversals per poll, which is the shape that made the
// old per-row relation filter O(N·edges) (see matchRelationFilter's doc).
//
// # Row visibility is applied downstream, not here
//
// The ids returned are unfiltered by ACL. That is safe because the caller
// intersects them with `src.listType`, which IS ACL-scoped — an id the
// principal cannot read is absent from that list and never reaches the output.
// Gating here too would be a second, redundant read of every neighbor.
func (b *caldavBackend) dynamicMembers(
	ctx context.Context, name string,
) (members map[string]struct{}, scoped bool, err error) {
	dyn, driverID, ok := b.resolveDynamic(ctx, name)
	if !ok {
		return nil, false, nil
	}
	dir := store.DirectionOutgoing
	if dyn.Direction.IsIncoming() {
		dir = store.DirectionIncoming
	}
	// The edge runs member→driver by default, so from the DRIVER's side the
	// query is the mirror of the configured direction.
	q := store.RelationQuery{EntityID: driverID, Type: dyn.Relation, Direction: flipDirection(dir)}
	rels, err := listRelationsCtx(ctx, b.app.Services().Store, q)
	if err != nil {
		return nil, false, fmt.Errorf("caldav %q: driver relations: %w", name, err)
	}
	members = make(map[string]struct{}, len(rels))
	for _, r := range rels {
		id := r.From
		if dir == store.DirectionIncoming {
			id = r.To
		}
		members[id] = struct{}{}
	}
	return members, true, nil
}

// flipDirection returns the mirror direction, so a member→driver edge can be
// queried from the driver's side.
func flipDirection(d store.Direction) store.Direction {
	if d == store.DirectionOutgoing {
		return store.DirectionIncoming
	}
	return store.DirectionOutgoing
}

// listTodos projects a collection's matching entities into to-dos, ACL-scoped.
//
// Reads go through feedEntitySource, the same ACL-gated source the ICS feed
// uses: an entity the principal cannot read is simply absent from the
// collection, not a 403 (which would be an existence oracle).
func (b *caldavBackend) listTodos(ctx context.Context, name string) ([]caldav.CalendarObject, error) {
	m, cfg, err := b.mapperFor(ctx, name)
	if err != nil {
		return nil, err
	}
	src := feedEntitySource{app: b.app}
	ents, err := src.listType(ctx, cfg.EntityType)
	if err != nil {
		return nil, err
	}
	filters, err := filter.ParseAll(cfg.Where)
	if err != nil {
		return nil, fmt.Errorf("caldav %q: %w", name, err)
	}
	members, scoped, err := b.dynamicMembers(ctx, name)
	if err != nil {
		return nil, err
	}
	entDef, _ := b.app.State().Meta.GetEntityDef(cfg.EntityType)

	out := make([]caldav.CalendarObject, 0, len(ents))
	for _, e := range ents {
		// Membership first: for a dynamic collection this is the cheap set
		// lookup, and it excludes most of the type before any filter runs.
		if scoped {
			if _, isMember := members[e.ID]; !isMember {
				continue
			}
		}
		matched, matchErr := filter.MatchAll(entityRecord(e), filters, entDef, b.app.State().Meta)
		if matchErr != nil {
			return nil, fmt.Errorf("caldav %q: %w", name, matchErr)
		}
		if !matched {
			continue
		}
		obj, buildErr := b.objectFor(ctx, name, m, e)
		if buildErr != nil {
			return nil, buildErr
		}
		out = append(out, obj)
	}
	return out, nil
}

// objectFor renders one entity as a CalDAV resource.
//
// The href is the ALIAS's href when the resource originated in a client (so
// the client keeps addressing it by the name it chose), and a derived one
// otherwise. Preserving a client-chosen href is what stops a synced to-do from
// being re-created as a duplicate.
func (b *caldavBackend) objectFor(
	ctx context.Context, collection string, m *caldavMapper, e *entitypkg.Entity,
) (caldav.CalendarObject, error) {
	uid := feedUID(e.Type, e.ID)
	href := uid + ".ics"
	if alias, ok := b.app.caldavAliases.LookupByEntity(aliasPrincipal(ctx), collection, e.ID); ok {
		href = alias.Href
		uid = alias.UID
	}

	obj, err := b.renderObject(ctx, collection, m, e, href, uid)
	if err != nil {
		return caldav.CalendarObject{}, err
	}
	return *obj, nil
}

// aliasPrincipal is the identity an alias is keyed under.
//
// The stamped principal's User — on the Pratique path that is the verified JWT
// subject (`usr_…`), not the email, so the key does not drift when someone
// changes their address. An unauthenticated deployment yields "", which is a
// legitimate single-user key space rather than a special case.
func aliasPrincipal(ctx context.Context) string {
	return principal.From(ctx).User
}

// fieldRedactor is the `visible:` redactor applied on the read path.
//
// A method rather than a direct appRedactor call so a test can substitute one:
// the wiring (renderObject actually redacting) is the part worth pinning, and
// standing up a full ACL policy to prove it would test the resolver instead.
func (b *caldavBackend) fieldRedactor() visibility.FieldRedactor {
	if b.redactor != nil {
		return b.redactor
	}
	return appRedactor(b.app)
}

// redactEntityFields returns a copy of e with `visible:`-hidden properties
// removed, or e unchanged when nothing is hidden.
//
// A COPY, never a mutation: the entity comes from the shared store snapshot, so
// deleting properties in place would redact it for every other reader in the
// process — including write-prep reads, where a missing property is an ERASURE
// (see the PatchEntity rule in CLAUDE.md).
//
// Content is cleared alongside the properties when the collection maps
// DESCRIPTION to the body: `visible:` names properties, so it has no vocabulary
// for the body, and a redactor that hid `body` while the mapping routed the
// markdown body into DESCRIPTION would leak exactly what it was asked to hide.
// Erring toward hiding is the safe direction for a read path.
func redactEntityFields(
	ctx context.Context, r visibility.FieldRedactor, e *entitypkg.Entity,
) *entitypkg.Entity {
	if r == nil || e == nil {
		return e
	}
	hidden := r.HiddenProperties(ctx, e)
	if len(hidden) == 0 {
		return e
	}
	clone := *e
	clone.Properties = make(map[string]any, len(e.Properties))
	for k, v := range e.Properties {
		if _, hide := hidden[k]; hide {
			continue
		}
		clone.Properties[k] = v
	}
	if _, hide := hidden[dataentryconfig.CalDAVDescriptionBody]; hide {
		clone.Content = ""
	}
	return &clone
}

// renderObject is the SINGLE place an entity becomes a CalDAV resource.
//
// Both the listing path and the post-write response go through it, so the ETag
// a client receives from a PUT is computed over exactly the rendering a later
// GET returns. Two call sites deriving that independently is how a conditional
// request starts failing for no visible reason.
func (b *caldavBackend) renderObject(
	ctx context.Context, collection string, m *caldavMapper, e *entitypkg.Entity, href, uid string,
) (*caldav.CalendarObject, error) {
	// Field-level `visible:` redaction, applied HERE because this is the single
	// place an entity becomes a CalDAV resource.
	//
	// visibleReader gates ROWS — a hidden entity never reaches this function.
	// It does not touch FIELDS, and docs/acl-security.md commits to redaction on
	// "every HTTP read shape". CalDAV was a new read shape that skipped it: a
	// collection mapping `description: body` served the body verbatim to a
	// principal whose role redacts it, because toTodo reads e.GetString/e.Content
	// straight off the raw entity.
	//
	// Redacting before the mapping (rather than after) means a hidden property
	// is simply absent, and the mapper's existing "absent property → omit the
	// iCalendar property" rule does the rest — no CalDAV-specific redaction
	// logic, and no way for a hidden value to survive into the ETag either.
	e = redactEntityFields(ctx, b.fieldRedactor(), e)
	td := m.toTodo(e, uid, m.link(e.Type, e.ID))
	ic := calfeed.ICal{Now: time.Now().UTC()}
	data, err := icalFromTodo(ic, td)
	if err != nil {
		return nil, err
	}
	return &caldav.CalendarObject{
		Path:    b.calendarPath(collection) + href,
		ModTime: e.UpdatedAt,
		ETag:    strings.Trim(ic.TodoETag(td), `"`),
		Data:    data,
	}, nil
}

func (b *caldavBackend) ListCalendarObjects(
	ctx context.Context, p string, _ *caldav.CalendarCompRequest,
) ([]caldav.CalendarObject, error) {
	name, _, ok := b.splitPath(p)
	if !ok {
		return nil, webdav.NewHTTPError(http.StatusNotFound, errors.New("caldav: unknown collection"))
	}
	return b.listTodos(ctx, name)
}

// QueryCalendarObjects answers a calendar-query REPORT.
//
// Every object is listed and go-webdav's exported Filter applies the RFC 4791
// predicate. That is what makes the canonical "pending to-dos" query (§7.8.9,
// which keys on COMPLETED being ABSENT) work without rela implementing filter
// semantics itself.
func (b *caldavBackend) QueryCalendarObjects(
	ctx context.Context, p string, q *caldav.CalendarQuery,
) ([]caldav.CalendarObject, error) {
	objs, err := b.ListCalendarObjects(ctx, p, nil)
	if err != nil {
		return nil, err
	}
	return caldav.Filter(q, objs)
}

func (b *caldavBackend) GetCalendarObject(
	ctx context.Context, p string, _ *caldav.CalendarCompRequest,
) (*caldav.CalendarObject, error) {
	name, href, ok := b.splitPath(p)
	if !ok || href == "" {
		return nil, webdav.NewHTTPError(http.StatusNotFound, errors.New("caldav: not found"))
	}
	objs, err := b.listTodos(ctx, name)
	if err != nil {
		return nil, err
	}
	want := b.calendarPath(name) + href
	for i := range objs {
		if objs[i].Path == want {
			return &objs[i], nil
		}
	}
	return nil, webdav.NewHTTPError(http.StatusNotFound, errors.New("caldav: not found"))
}

// CreateCalendar is refused.
//
// Collections are operator-declared config, so a client-minted one would have
// no mapping and become an orphan the server could never serve. Apple's
// Calendar.app issues MKCALENDAR unprompted on account setup, so this is
// reached in practice — refusing is the correct answer, not an omission.
func (b *caldavBackend) CreateCalendar(context.Context, *caldav.Calendar) error {
	return webdav.NewHTTPError(http.StatusMethodNotAllowed,
		errors.New("caldav: collections are declared in data-entry.yaml, not created by clients"))
}

// watermarkCTag derives the ctag from the store's entity-type watermark instead
// of rendering the collection, when the backend supports one.
//
// ok=false means "no watermark available" — the caller falls back to the
// content-derived tag. That is the fsstore path (no monotonic sequence) and it
// stays correct, just not cheap.
//
// # Why a type watermark is a SOUND ctag, despite being coarser
//
// The contract a ctag must satisfy is one-directional: it MUST change when the
// collection's content changes. It is explicitly allowed to change when nothing
// a client can see changed — RFC-wise the client simply re-enumerates and finds
// the same ETags.
//
// The watermark moves on any write to the entity type. Every change to this
// collection is such a write, so the required direction holds. It also moves for
// writes the collection filters out, for entities this principal cannot read,
// and for other collections over the same type — all spurious re-enumerations,
// none of them incorrect.
//
// The reverse trade is what makes this safe: a missed change strands a client
// forever, a spurious one costs it a single listing.
//
// # What this does NOT cover
//
// Config changes. The watermark is over entity rows, so editing the collection's
// `where:` clause or its property mapping does not move it, and a client keeps
// its stale view until the next entity write. The rendered tag included the
// mapping implicitly, so this is a real (if minor) regression: an operator who
// edits a mapping and wants clients to see it immediately should touch an entity
// or restart. Folding a config generation into the tag is the fix if that
// becomes a problem.
func (b *caldavBackend) watermarkCTag(
	ctx context.Context, name string,
) (ctag string, supported bool, err error) {
	_, cfg, err := b.mapperFor(ctx, name)
	if err != nil {
		return "", false, err
	}
	wm, ok := b.app.Services().Store.(store.TypeWatermark)
	if !ok {
		return "", false, nil
	}
	seq, err := wm.EntityTypeWatermark(ctx, cfg.EntityType)
	if err != nil {
		return "", false, err
	}
	// Namespaced by collection so two collections over the same type do not
	// share a tag: they render different content, and a client that switched
	// between them must not see a matching tag. The seq alone would collide.
	h := sha256.New()
	fmt.Fprintf(h, "wm:%d:%s:%d", len(name), name, seq)
	sum := h.Sum(nil)
	return `"` + base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(sum[:10]) + `"`, true, nil
}

// collectionCTag returns the collection ctag: a tag over every entry a client
// would currently receive, so it changes exactly when the collection's content
// does. See withCTag for why this is content-derived rather than counter-derived.
//
// Hashes the per-resource ETags that listTodos already computed, so the ctag is
// defined over precisely the bytes a client would be served — including a
// deletion, which drops an ETag out of the hash. Length-prefixed to avoid
// boundary ambiguity between adjacent tags, matching calfeed.CollectionTag.
func (b *caldavBackend) collectionCTag(ctx context.Context, name string) (string, error) {
	if tag, ok, err := b.watermarkCTag(ctx, name); err != nil {
		return "", err
	} else if ok {
		return tag, nil
	}
	objs, err := b.listTodos(ctx, name)
	if err != nil {
		return "", err
	}
	h := sha256.New()
	for i := range objs {
		fmt.Fprintf(h, "%d:%s", len(objs[i].ETag), objs[i].ETag)
	}
	sum := h.Sum(nil)
	return `"` + base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(sum[:10]) + `"`, nil
}
