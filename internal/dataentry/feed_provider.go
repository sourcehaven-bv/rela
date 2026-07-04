package dataentry

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/calfeed"
	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/filter"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// feedProvider is the internal, transport-independent view of a calendar feed:
// enumerate its events and fetch one by UID. Both the ICS/JSON export (which
// calls List) and a future read-only CalDAV server (which additionally calls
// Get per resource) drive off this single abstraction. rela derives ETags,
// collection tags, and the sync cursor from the events themselves — a provider
// only produces events.
type feedProvider interface {
	// List enumerates the feed's events. opts.Since, when non-nil, is an
	// advisory cursor from a previous List; a provider may use it to return
	// only changed events. The returned cursor is the provider's watermark
	// (or empty when it does not support delta), round-tripped back as
	// opts.Since next time.
	List(ctx context.Context, opts feedListOpts) (events []calfeed.Event, cursor string, err error)
	// Get returns one event by UID, or ok=false if it is not in the feed.
	Get(ctx context.Context, uid string) (ev calfeed.Event, ok bool, err error)
}

// feedListOpts carries advisory delta-sync state into feedProvider.List.
type feedListOpts struct {
	// Since is an opaque cursor previously returned by List (empty on first or
	// full sync).
	Since string
}

// entitySource is the minimum entity access a declarative feed needs, declared
// at the call site (per CLAUDE.md). The production implementation lists under
// the ACL read gate and loads single entities via the visible reader; tests
// supply a fake.
type entitySource interface {
	// listType returns all entities of a type the caller may read, already
	// ACL-scoped.
	listType(ctx context.Context, entityType string) ([]*entity.Entity, error)
	// getEntity returns one entity the caller may read, or ok=false.
	getEntity(ctx context.Context, entityType, id string) (e *entity.Entity, ok bool, err error)
}

// deepLinker builds an app URL for an entity (mirrors rela.url on the Lua side).
type deepLinker func(entityType, id string) string

// declarativeFeed is a feedProvider synthesized from a dataentryconfig.Feed. It
// runs each source's filter over ACL-scoped entities and maps matching
// entities' properties to all-day calendar events.
type declarativeFeed struct {
	cfg    dataentryconfig.Feed
	meta   *metamodel.Metamodel
	src    entitySource
	link   deepLinker
	feedID string // the config key; used as the default calendar name
}

// newDeclarativeFeed builds a provider for one configured feed. It pre-parses
// each source's filter clauses so a malformed clause is caught before any
// entity is read (config validation catches this earlier still).
func newDeclarativeFeed(feedID string, cfg dataentryconfig.Feed, meta *metamodel.Metamodel, src entitySource, link deepLinker) (*declarativeFeed, error) {
	// Validate sources reference known types up front so List/Get can assume it.
	for i, s := range cfg.Sources {
		if _, ok := meta.GetEntityDef(s.EntityType); !ok {
			return nil, fmt.Errorf("feed %q: source[%d]: unknown entity type %q", feedID, i, s.EntityType)
		}
		if _, err := filter.ParseAll(s.Where); err != nil {
			return nil, fmt.Errorf("feed %q: source[%d]: %w", feedID, i, err)
		}
	}
	return &declarativeFeed{cfg: cfg, meta: meta, src: src, link: link, feedID: feedID}, nil
}

// calendarName is the feed's display name: the configured meta.name, or the
// feed's config key as a fallback.
func (d *declarativeFeed) calendarName() string {
	if d.cfg.Meta.Name != "" {
		return d.cfg.Meta.Name
	}
	return d.feedID
}

// List implements feedProvider. It enumerates every source, maps matching
// entities to events, and returns the max entity modification time as the
// cursor. opts.Since is honored as an extra "modified since" gate so a client
// re-poll only re-emits changed events; unchanged content still yields a stable
// ETag downstream, so ignoring or honoring Since is equally correct.
func (d *declarativeFeed) List(ctx context.Context, opts feedListOpts) ([]calfeed.Event, string, error) {
	var since time.Time
	if opts.Since != "" {
		// Cursor is an RFC3339 timestamp we minted last time; tolerate a parse
		// failure by treating it as a full sync (correctness over efficiency).
		if t, err := time.Parse(time.RFC3339Nano, opts.Since); err == nil {
			since = t
		}
	}

	var events []calfeed.Event
	var watermark time.Time
	for _, s := range d.cfg.Sources {
		ents, err := d.src.listType(ctx, s.EntityType)
		if err != nil {
			return nil, "", err
		}
		filters, err := filter.ParseAll(s.Where)
		if err != nil {
			return nil, "", err
		}
		entDef, _ := d.meta.GetEntityDef(s.EntityType)
		for _, e := range ents {
			if !since.IsZero() && !e.UpdatedAt.After(since) {
				continue
			}
			ev, ok, err := d.mapEntity(e, s, entDef, filters)
			if err != nil {
				return nil, "", err
			}
			if !ok {
				continue
			}
			events = append(events, ev)
			if e.UpdatedAt.After(watermark) {
				watermark = e.UpdatedAt
			}
		}
	}
	cursor := opts.Since
	if !watermark.IsZero() {
		cursor = watermark.UTC().Format(time.RFC3339Nano)
	}
	return events, cursor, nil
}

// Get implements feedProvider: it finds the source whose type matches the UID's
// type prefix, loads that entity, re-applies the source filter, and maps it.
func (d *declarativeFeed) Get(ctx context.Context, uid string) (calfeed.Event, bool, error) {
	entityType, id, ok := splitFeedUID(uid)
	if !ok {
		return calfeed.Event{}, false, nil
	}
	for _, s := range d.cfg.Sources {
		if s.EntityType != entityType {
			continue
		}
		e, found, err := d.src.getEntity(ctx, entityType, id)
		if err != nil {
			return calfeed.Event{}, false, err
		}
		if !found {
			return calfeed.Event{}, false, nil
		}
		filters, err := filter.ParseAll(s.Where)
		if err != nil {
			return calfeed.Event{}, false, err
		}
		entDef, _ := d.meta.GetEntityDef(entityType)
		ev, mapped, err := d.mapEntity(e, s, entDef, filters)
		if err != nil {
			return calfeed.Event{}, false, err
		}
		return ev, mapped, nil
	}
	return calfeed.Event{}, false, nil
}

// mapEntity applies a source's filter to one entity and, if it matches and has
// a date value, maps its properties to a calendar event. ok=false means the
// entity is filtered out or has no usable date (it is silently skipped, not an
// error — a task with no due date is simply not on the calendar).
func (d *declarativeFeed) mapEntity(e *entity.Entity, s dataentryconfig.FeedSource, entDef *metamodel.EntityDef, filters []*filter.Filter) (calfeed.Event, bool, error) {
	matched, err := filter.MatchAll(entityRecord(e), filters, entDef, d.meta)
	if err != nil {
		return calfeed.Event{}, false, err
	}
	if !matched {
		return calfeed.Event{}, false, nil
	}

	dateStr := e.GetString(s.Date)
	if dateStr == "" {
		return calfeed.Event{}, false, nil // no date → not on the calendar
	}
	dateDef := entDef.Properties[s.Date]
	day, err := metamodel.ParseDateValue(dateStr, &dateDef)
	if err != nil {
		// An unparseable date on one entity (e.g. a bad hand-edit) skips that
		// event rather than failing the whole feed — consistent with rela's
		// "tolerate temporarily invalid data" write policy.
		return calfeed.Event{}, false, nil //nolint:nilerr // skip malformed entity, not a feed error
	}

	summaryProp := s.Summary
	if summaryProp == "" {
		summaryProp = entDef.GetPrimaryProperty()
	}

	ev := calfeed.Event{
		UID:     feedUID(e.Type, e.ID),
		Summary: e.GetString(summaryProp),
		Start:   day,
		URL:     d.link(e.Type, e.ID),
	}
	if s.Description != "" {
		ev.Description = e.GetString(s.Description)
	}
	if s.EndDate != "" {
		if endStr := e.GetString(s.EndDate); endStr != "" {
			endDef := entDef.Properties[s.EndDate]
			if end, err := metamodel.ParseDateValue(endStr, &endDef); err == nil {
				ev.End = end
			}
		}
	}
	ev.RRule = resolveRRule(s.Rrule, e)
	if s.Alarm != "" {
		ev.Alarms = []calfeed.Alarm{{Trigger: s.Alarm}}
	}
	return ev, true, nil
}

// resolveRRule turns a source's rrule config into the event's recurrence rule.
// A value containing "=" is a literal rule used as-is; a bare identifier is a
// property name read from the entity (empty when the property is unset).
func resolveRRule(cfg string, e *entity.Entity) string {
	switch {
	case cfg == "":
		return ""
	case strings.Contains(cfg, "="):
		return cfg
	default:
		return e.GetString(cfg)
	}
}

// renderFeed collects all events from a provider and packages them with the
// calendar metadata for serialization.
func (d *declarativeFeed) renderFeed(ctx context.Context) (calfeed.Feed, error) {
	events, _, err := d.List(ctx, feedListOpts{})
	if err != nil {
		return calfeed.Feed{}, err
	}
	return calfeed.Feed{
		Name:        d.calendarName(),
		Description: d.cfg.Meta.Description,
		Color:       d.cfg.Meta.Color,
		Events:      events,
	}, nil
}
