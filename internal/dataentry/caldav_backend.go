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
func (b *caldavBackend) ListCalendars(_ context.Context) ([]caldav.Calendar, error) {
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
	return out, nil
}

func (b *caldavBackend) GetCalendar(_ context.Context, p string) (*caldav.Calendar, error) {
	name, _, ok := b.splitPath(p)
	if !ok {
		return nil, webdav.NewHTTPError(http.StatusNotFound, errors.New("caldav: unknown collection"))
	}
	cfg, found := b.app.State().Cfg.CalDAV.Static[name]
	if !found {
		return nil, webdav.NewHTTPError(http.StatusNotFound, errors.New("caldav: unknown collection"))
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

// mapperFor resolves the mapper for a collection, or a 404.
func (b *caldavBackend) mapperFor(name string) (*caldavMapper, dataentryconfig.CalDAVCollection, error) {
	s := b.app.State()
	cfg, ok := s.Cfg.CalDAV.Static[name]
	if !ok {
		return nil, cfg, webdav.NewHTTPError(http.StatusNotFound, errors.New("caldav: unknown collection"))
	}
	link := func(entityType, id string) string {
		return b.baseURL + "/entity/" + entityType + "/" + id
	}
	return newCalDAVMapper(name, cfg, s.Meta, link), cfg, nil
}

// listTodos projects a collection's matching entities into to-dos, ACL-scoped.
//
// Reads go through feedEntitySource, the same ACL-gated source the ICS feed
// uses: an entity the principal cannot read is simply absent from the
// collection, not a 403 (which would be an existence oracle).
func (b *caldavBackend) listTodos(ctx context.Context, name string) ([]caldav.CalendarObject, error) {
	m, cfg, err := b.mapperFor(name)
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
	entDef, _ := b.app.State().Meta.GetEntityDef(cfg.EntityType)

	out := make([]caldav.CalendarObject, 0, len(ents))
	for _, e := range ents {
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

// collectionCTag returns the collection ctag: a tag over every entry a client
// would currently receive, so it changes exactly when the collection's content
// does. See withCTag for why this is content-derived rather than counter-derived.
//
// Hashes the per-resource ETags that listTodos already computed, so the ctag is
// defined over precisely the bytes a client would be served — including a
// deletion, which drops an ETag out of the hash. Length-prefixed to avoid
// boundary ambiguity between adjacent tags, matching calfeed.CollectionTag.
func (b *caldavBackend) collectionCTag(ctx context.Context, name string) (string, error) {
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
