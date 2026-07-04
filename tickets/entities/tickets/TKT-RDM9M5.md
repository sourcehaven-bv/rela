---
id: TKT-RDM9M5
type: ticket
title: 'Phase 1: declarative calendar/feed export — internal/feed serializer, feeds: config (multi-source), ICS+JSON HTTP endpoint'
kind: enhancement
priority: medium
effort: l
status: in-progress
---

## Description

Implement **Phase 1** of the calendar/feed export, **declarative-first**. Design
in **RES-AHY3VS** (feed model + renderers), **RES-1X4NS9** (auth/transport), and
the declarative-schema + provider-contract design notes. Implements
**FEAT-OT4361**.

### Deliverables (Phase 1)

1. **`internal/feed`** — leaf domain package: `Feed`/`Event`/`Alarm` model +
**event-granular CalDAV-ready** iCal serializer (`RenderEvent`,
`RenderCollection`, `ETag`, `CollectionTag`; VEVENT + VALARM, hand-rolled, no
vendor) + JSON. **All-day events only** in Phase 1 (`VALUE=DATE`) — see the
datetime dependency below. Pure model→bytes; no `store`/`metamodel` import.
2. **Declarative `feeds:` config** on `dataentryconfig.Config` — a feed is
`{ meta?, sources: [ … ] }`; each **source** is a single-entity-type projection:
`entity_type` + `where` (list of ANDed `internal/filter` clauses)
   + date mapping (`date:` prop; `start:`/`end:` range later needs datetime) +
`summary:`/`description:`/`alarm:` prop mappings. Multiple sources merge into
one calendar (also the OR mechanism, since the filter has no OR). UID =
`<type>-<id>@<domain>` (CalDAV-unique + routable). Mirrors
`dataentryconfig.List` shape + `validateLists` fail-fast metamodel validation.
3. **Feed synthesizer** — compiles a declarative feed into the internal
`{ list, get }` provider abstraction: `list` runs each source's
`ListEntities`+`filter.MatchAll` (the TYPED path) and maps props → events; rela
derives ETag/`CollectionTag`/cursor (`max(modified)`) itself. One provider → ICS
+ JSON now, CalDAV later, all free.
4. **HTTP endpoint** `GET /api/v1/_feeds/{name}.{ics,json}` on the **inner `/api/`
mux** (inherits ACL read gate + CSRF chain), CSRF-exempt via
`nonBrowserExemptPrefixes`, **loopback-trust only** (no feed auth) with a
**principal-resolution seam**.

Events deep-link via the existing `rela.url("/entity/<type>/<id>")`.

### Key decisions

- **Declarative-first; Lua deferred.** Research (rela's `internal/filter` +
`GraphQuery`) shows the declarative 80% is large: typed date comparison,
existence (`prop!=`), glob/regex, multi-source OR, relation-selection (future
`traverse:`). Lua provider is a later escape hatch for computed/2-events/graph
cases; it compiles into the SAME `{list,get}` provider (RES-1X4NS9), so nothing
is wasted.
- **All-day only in Phase 1.** rela has no `datetime` type (only `date`); a `date`
field gives a date-picker, so timed values can't be authored cleanly. Timed
events (`DTSTART` with time; start/end range) depend on **TKT-ZYOBSN (add a
`datetime` type)** and are picked up type-driven once it lands. All-day covers
the PIM nudge (a task is due *on a day*).
- **Auth (RES-1X4NS9):** loopback-only Phase 1; networked = future pratique
scoped-token + read-only CalDAV (pratique FR filed). Fail-loud guard dropped
(RR-7C151B wont-fix). Provider-return replaces the stdout seam (RR-78OHN5
wont-fix).

### Out of scope (later)

- **Lua feed provider** (escape hatch for computed / 2-events-per-entity / graph
cases) — compiles to the same provider.
- **Timed events** — blocked on TKT-ZYOBSN (`datetime` type).
- **Relation-based sources** (`traverse:` per source, mirroring `ViewTraverse`).
- **Read-only CalDAV endpoint + pratique-token auth** — built on Phase 1's
event-granular serializer + provider.
- **CalDAV two-way sync + `VTODO`** (Phase 2); Atom/RSS; `RRULE`; swiftbar/`launchd`.
