---
id: TKT-JPDXMO
type: ticket
title: 'CalDAV: graph-driven collections — one per entity (a collection per project)'
kind: enhancement
priority: medium
effort: l
status: backlog
---

## Problem

CalDAV collections are static config keys today: `caldav.<name>` is one entry,
`splitPath` maps a URL segment to it, and `ListCalendars` iterates the config
map. So a project-per-list layout — the natural shape for a task tracker — means
hand-writing one config block per project and editing YAML whenever a project is
added.

## Proposal

Let a collection be generated from the GRAPH: each entity of a chosen type
becomes a collection, carrying the entities linked to it.

The config splits into two named siblings, so a reader can tell at a glance
which keys are collections and which are patterns that expand into several:

```yaml
caldav:
  static:
    tasks:                       # one collection, at /calendars/tasks/
      entity_type: task
      meta: {name: "rela Tasks"}
      summary: title
      completion: {...}
  dynamic:
    project_tasks:               # a PATTERN — expands to one collection
      entity_type: task          # per project entity
      driver_type: project
      relation: belongs-to
      where: ["status != cancelled"]
      summary: title
      completion: {...}
```

`static:`/`dynamic:` are BOTH named on purpose. An earlier sketch put the
dynamic block beside an unlabelled `caldav:`, which left the common case unnamed
— a reader meeting `caldav:` cold could not tell there was another kind until
they happened to see the sibling. Naming both makes the pairing self-describing.

The driver fields are flat (`driver_type` + `relation`) rather than nested under
a `per:` block: under a heading already called `dynamic:` the extra nesting
restates what the block name says.

### The key stays, and it is the URL segment

`project_tasks` yields `project_tasks--PROJ-1`, `project_tasks--PROJ-2`, …

Deriving the segment from `entity_type` instead (so `task--PROJ-1`) would drop
the key entirely and reads more cleanly, but it collides the moment two patterns
share a driver — tasks-per-project AND bugs-per-project both want a segment from
`project`. A map keyed by name makes that collision impossible by construction
(YAML rejects duplicate keys), where a list would only catch it in validation.

This also preserves the property the static block already has: **the YAML key is
the id** (the URL segment and the alias key), and `meta.name` is the label. The
key must stay stable because users paste the URL into clients; the label is free
to change.

## Why this is more than a config change

Several things need deciding before writing code.

- **The composite segment is FORCED, not chosen.** go-webdav classifies a
resource by its depth below the mount prefix (root / principal / home-set /
calendar / object), so `calendars/project_tasks/PROJ-1/` would be read as an
OBJECT, not a collection. The pattern and the driver id must share one segment.
`--` is already the repo's unambiguous separator (`feedUIDSep`) and entity ids
are pinned to `^[A-Za-z0-9][A-Za-z0-9_-]*$`, so the segment needs no escaping —
which matters, because collection URLs must stay human-typable (Thunderbird
requires pasting one by hand).
- **Href stability is load-bearing.** A collection URL must never change or
clients re-add the entire list as new. Project *ids* are stable, titles are not,
so the id has to be in the path segment even though a slug reads better. A
rename must NOT move the collection — the same lesson the alias table encodes
for resources, one level up.
- **A deleted project deletes a collection.** Clients handle a vanished
COLLECTION far less gracefully than a vanished resource, and behaviour differs
per client. Needs empirical testing before shipping, not after.
- **Cost multiplies by collection count.** A poll currently re-renders one
collection; with P projects and T tasks a Depth:1 home-set PROPFIND is O(P·T),
because `ctagResolver` memoizes per href but each computation re-scans the whole
entity type underneath. The ctag work (TKT-MF1CWZ) assumed a handful of
collections. Either a shared per-request "tasks grouped by driver" index lands
first, or this is a scalability regression disguised as a feature.
- **A client-created to-do needs a RELATION, and no config construct exists.**
`createFromTodo` builds an entity from `defaults:`, which is a map of literal
property values only. A to-do created inside `project_tasks--PROJ-1` must also
get a `belongs-to` edge to PROJ-1, or it lands in the type but in no collection
and vanishes from the client on the next sync. This is a genuine new config
surface, not a wiring detail.
- **Collection names stop being public config keys and become entity ids.**
`internal/dataentry/CLAUDE.md` records that config names are not secret while
entity existence is. A segment like `project_tasks--PROJ-SECRET` puts an entity
id in a URL, a client sync log, and any error message. Enumeration must become
principal-dependent (`feedEntitySource.listType` on the driver type), and an
unknown-vs-hidden collection must answer a uniform 404 — never a named 403,
which is the right answer for a config-declared capability but not for an id.

## Interactions

- **Every generated collection must advertise `<C:comp name="VTODO"/>`** —
Tasks.org silently hides collections that do not, with no error message. See
`docs/caldav-clients.md`.
- **The alias table is keyed by `(collection, href)`**, so a moving collection
name would orphan every alias under it. Another reason ids, not slugs. Aliases
also become orphaned when a driver entity is deleted.
- **Colour has a natural home here** (TKT-GFLSFP): a `project` entity can carry
a colour property, so the collection colour comes from the graph rather than
config. That does not make the colour per-USER — see the per-user state ticket.
- **ACL**: a principal who cannot read a project must not see its collection at
all. Collection enumeration becomes an ACL-gated read, which it is not today.
- **`mapperFor` loses its ctx-free purity.** It is the chokepoint for
`listTodos`, `PutCalendarObject` and `DeleteCalendarObject`, and a dynamic
resolver needs `ctx` for the ACL-gated driver lookup. Mechanical, but it ripples
through the whole write path.
- **This would be the first config construct in the repo where one entry expands
to N runtime things driven by graph data.** `ViewConfig` is the closest
precedent (entity-typed entry + traversal rules, instantiated per entity) but
its instances are selected BY URL, never enumerated by the server. CalDAV forces
enumeration, because PROPFIND Depth:1 on the home set is how clients discover
collections — there is no "type the URL for project X" affordance in Reminders.

## Acceptance criteria

1. Each entity of `driver_type:` yields one collection, discovered by a client
from one account URL.
2. The collection href is derived from the pattern key and the entity ID, and
survives a rename.
3. Renaming the entity updates the display name without moving the collection.
4. Deleting the entity removes the collection; the observed client behaviour is
documented (not assumed).
5. Collections a principal cannot read are absent from enumeration, and an
unknown-vs-hidden collection is indistinguishable (uniform 404).
6. Every generated collection advertises its supported component set.
7. A client-created to-do inside a generated collection gets the driver relation
and remains in that collection on the next sync.
8. Poll cost is measured before/after with P collections, and the shared
grouping index keeps it off O(P·T).
9. Existing `caldav:` configs are migrated to `caldav.static:` — this ticket
lands the nesting, so it must ship with the migration or before any release that
exposes the old shape.

## Implementation status (2026-08-17): read path landed, ticket stays open

**Status stays `backlog` deliberately.** Read and create both work now, but AC4
(driver deletion) is unverified against real clients and the AC requires that it
be observed rather than assumed. `in-progress` would be the honest label, but
the workflow requires a terminal status to merge; `backlog` says "more to do
here" without claiming the feature is finished. Split that way deliberately: the read path is testable against real clients,
which is how every genuine CalDAV bug in this arc was found, and the write half
carries an unresolved atomicity question (below).

### Landed

- **AC1** `caldav.dynamic:` patterns expand to one collection per driver entity.
- **AC2/AC3** The URL segment is `<pattern>--<driverID>`, built from the ID, so a
  rename changes the display name (taken from the driver's title) without moving
  the collection. A moved href makes a client re-add the whole list as new.
- **AC5** Enumeration is per-principal: patterns expand only over drivers the
  caller may read, and an absent OR hidden driver returns the same 404. The
  driver id sits in the URL, so a distinguishable status would be an existence
  oracle for it.
- **AC6** Generated collections advertise their component set — inherited from
  the embedded `CalDAVCollection`, so it cannot drift from the static path.
- **AC8** Solved a level down, in the store rather than here.
  `store.TypeWatermark` answers "has anything of this type changed?" with an
  index-only `max(seq)`, replacing a full collection render on every `getctag`
  poll. Deletion tombstones are included: `max(seq)` over live rows alone goes
  DOWN when the newest row is hard-deleted, and a tag that moves backwards
  strands every client that already saw the higher value. pgstore-only,
  type-asserted; fsstore keeps the content-derived tag.

Membership is ONE relation traversal anchored on the driver, not one per member
— O(1) queries per collection rather than O(members), the shape that made the
old per-row relation filter O(N·edges).

### Landed since

- **AC7 (client-created entries)** — `createFromTodo` now gives a new entry the
  driver relation, so it is a member of the collection it was created in. The
  config already carried `relation:` (it serves both directions by design), so
  no new config surface was needed.

  **Atomicity resolved as a compensating delete, not a transaction.**
  `entitymanager` exposes no `Tx`, and `store.Tx` would not have helped
  uniformly anyway: fsstore keeps writes already made (no rollback — a
  deliberate reduced guarantee), so the same compensation would be required
  there regardless. If the edge fails the entity is deleted, non-cascading —
  it is seconds old and its only possible edge is the one that just failed, so
  a cascade could only reach relations a concurrent writer added, which are not
  ours to remove. If the compensation itself fails the orphan survives and the
  error says so, which is the best available at this layer.

  The reasoning for deleting rather than keeping: an entity in the type but in
  no collection is invisible in every CalDAV view, so the user can neither find
  nor fix it. A failed create is visible and retryable; an orphan is neither.

### Deferred, with reasons

- **AC4 (driver deletion)** — deleting a driver removes a collection, and the
  ticket requires the client behaviour be *documented, not assumed*. That needs
  live observation against Reminders and Thunderbird; every client finding in
  this arc came from wire capture, and guessing here would violate the AC's own
  terms.
- **AC9 (migration)** — moot. The `caldav.static:` nesting shipped in #1308
  before any release exposed the flat shape, so there is nothing to migrate.

### Known regression, accepted

The rendered ctag covered the property mapping implicitly, so editing a
collection's `where:` or a property mapping moved it. The watermark is over
entity ROWS, so a config edit no longer does — a client keeps its stale view
until the next entity write. Recorded at `watermarkCTag`; the fix, if it bites,
is folding a config generation into the tag.
