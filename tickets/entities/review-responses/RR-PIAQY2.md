---
id: RR-PIAQY2
type: review-response
title: Source identity derived from content instead of position
finding: 'Sources were keyed on entity+date rather than index. Two sources over the same type differing only by where clauses — the documented way to express OR — collided: one rendered the other''s rows and colour and clobbered its cache entry. The existing multi-source test used two different types so it passed either way.'
severity: significant
resolution: 'Index is the identity: positional results plus a sourceIndex on CalendarEvent. Two tests added. The same pass fixed six smaller findings; see the body.'
status: addressed
---

## Finding

Several defects sharing one root cause: a source's identity was derived from its
**content** (`entity + ':' + date`) rather than its **position**.

Two sources over the same entity type and date property, differing only by
`where:`, is the documented way to express OR — the filter language has none, so
the config doc says "use a second source". Those two sources produced identical
keys, so:

- `Array.find` returned the first, and source #2 rendered source #1's rows
under source #1's colour.
- Both wrote `queryCache.setQueryData` under the same key, so one clobbered
the other.
- `longEventWarning` resolved `max_span` by entity type, hitting the same
first-match bug.

The existing multi-source test used two *different* entity types, so it passed
either way — which is why this survived.

## Resolution

Index is the identity. `fetched` is positional, `CalendarEvent` carries a
`sourceIndex`, and `longEventWarning` reads the source by that index. The event
id gained the index too, so two rows for the same entity from different sources
no longer collide.

Two tests added: distinct `where:` clauses reach distinct requests, and two
same-type sources render with their own colours.

## Also in this pass

- **`title` chip field**: the Go validator exempted `title` as an
"entity-level key", but the SPA renders a chip field from
`entity.properties[name]`. A type whose display property is `name` accepted
`property: title` at load and rendered nothing forever. Exemption dropped for
`title`, kept for `id`; both directions tested.
- **Locale-dependent sort**: the tiebreak used `localeCompare`, whose ordering
varies by runtime locale, while the comment claimed determinism. Now a plain
comparison.
- **Dead client-side defaults**: `?? 31`, `?? 4`, `?? 'blue'` duplicated
defaults the server already normalizes, so they would silently diverge.
- **`canCreate()`** asked an arbitrary entity whether the *type* may be
created, and returned true on an empty grid. It is an affordance, not a gate, so
it now simply reflects whether a create form is configured.
- **`parseClockMinutes`** rejected the `8:00` people actually type while
accepting `+8:00` (Atoi takes a sign). Replaced with `time.Parse`.
- **Underscore escape hatch** narrowed so it cannot swallow a real section name
(`_kanbans:` is a commented-out block, not an anchor).
