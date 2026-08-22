---
id: PLAN-Z636Y6
type: planning-checklist
title: 'Planning: Calendar views (month + week) for data-entry'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Problem:** rela can *export* time-bearing entities to foreign calendar clients
(`feeds:`, CalDAV) but cannot *show* a calendar inside the app. Dates deserve a
native spatial rendering for the same reason enums get kanban columns.

**Scope — IN:**
- `calendars:` block in `data-entry.yaml`, structurally mirroring `kanbans:`
- Month and week grids, next/prev/today navigation, "today" highlight
- Events from one or more `sources:` (multi-source merge, as feeds do)
- Click an event → entity page, or `edit_form` when configured
- Drag an event to another day → patch the date property (ACL-gated)
- Sidebar `calendar:` nav entry with count, gated by nav-entry `permission:`
- Server-side date-range windowing of the fetch

**Scope — OUT:** day/year views (TKT-S18C1U), ICS publishing of a calendar view,
CalDAV sync, recurrence expansion, drag-to-resize, sophisticated
overlapping-event layout.

**Acceptance criteria — three corrected during planning (see Approach §7).**

1. A `calendars:` entry renders at `/calendar/:id`, reachable from a sidebar `calendar:` nav entry.
2. Month view renders a full month grid (leading/trailing days of adjacent months shown but visually de-emphasised); week view renders 7 days. Both mark today and support next/prev/today.
3. Multiple `sources:` merge into one calendar.
4. `date`-typed sources render as all-day; `datetime`-typed as timed. `date` and `end_date` must be the same kind (feed rule, reused).
5. Clicking an event opens the entity, or `edit_form` if set.
6. Dragging an event patches the date property. Drag is **day-granular**: it changes the day and preserves the time-of-day. When `end_date` is set both move by the same whole-day delta, **preserving wall-clock start and end** (not elapsed duration — they differ across a DST boundary).
7. An entity the principal may not update is not draggable (`actionAllowed(entity,'update')`, the gate KanbanView.vue:44-46 uses).
8. **CORRECTED:** config load fails when `date`/`end_date` name a property that does not exist on the type, is not date/datetime-typed, is `list: true`, or mismatches kinds. (The original criterion said "non-writable" — there is no such concept; see Approach §7.)
9. Entities hidden by the read-side ACL never appear as events.
10. **CORRECTED:** `permission:` on the *navigation entry* gates the calendar (config.go:682). It is not a field on the calendar itself.
11. Source `where:` filters select the same set for a redacted and an unredacted principal (see the ordering discussion in §5).
12. A `datetime` source renders events at all — the single highest-value test in this ticket, guarding the `compareValues` defect (§5).
13. A timed event on the **last day** of the visible window renders.
14. The same instant renders in different cells under different display timezones, per the day-assignment invariant (§7).
15. Navigating month → next → previous serves correct data for each period (guards query-key aliasing, §6).

## Research

- [x] For larger features: run `/research` — **N/A**: the approach is fully
determined by two in-tree precedents; no unfamiliar subsystem, no competing
external libraries to survey.
- [x] Searched for existing libraries — see below
- [x] Checked codebase for similar patterns
- [x] Looked for reference implementations

**Research Doc:** N/A (RES-1Y2EB5 read for background on the calendar/CalDAV
arc)

**Existing Solutions:**

*Libraries:* deliberately **no** calendar library (FullCalendar, vue-cal,
v-calendar). Rejected: month/week grid geometry is ~200 lines of date math;
every candidate brings its own event model, styling system and drag
implementation that would have to be adapted to rela's config, ACL affordances
and SSE refetch. The project has no frontend calendar dependency today and
KanbanView.vue implements its own drag with plain HTML5 DnD — matching it keeps
one idiom. Date math uses the platform `Date`; no date library needed for
month/week arithmetic.

*In-tree patterns (verified file:line):*
- `internal/dataentryconfig/config.go:571-585` — `Kanban` struct; the shape to
mirror. Note it has **no** `permission:` field.
- `internal/dataentryconfig/config.go:1105-1134` — `FeedSource`: the exact
source-field vocabulary (`entity_type`, `where`, `date`, `end_date`, `summary`,
`description`) `CalendarSource` will copy.
- `internal/dataentryconfig/validate_feeds.go:74-175` — `validateFeeds` /
`validateFeedSource`: near line-for-line template for `validateCalendars`.
Includes the same-kind check (`feedKindMismatch`) and the display-property
fallback rule for an omitted `summary`.
- `internal/dataentry/feed_provider.go:236+` — `mapEntity`: the entity→event
projection, and the **filter-before-redact** ordering rule.
- `internal/dataentry/views_handler.go:284-291,461-465` — sidebar entry, href,
icon, ACL-scoped count for kanban.
- `frontend/src/views/KanbanView.vue` (1043 lines) — view structure, HTML5
drag, `canUpdate`, optimistic-update/SSE-echo handling (:73).
- `frontend/src/router/index.ts:57-62` — lazy route with `props: true`.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns
- [x] Alternatives considered
- [x] Dependencies identified

**Technical Approach**

**1. Config (`internal/dataentryconfig/config.go`)**

Add `Calendars map[string]Calendar` beside `Kanbans` / `Feeds` (~config.go:93).

**Tag convention trap:** `List` and `Kanban` both use yaml `entity_type` with
**json `entity`** (config.go:408, :572) — the SPA reads `config.entity`. Getting
this wrong is a silent client-side "unknown entity type". `CalendarSource` keeps
the field on the source, so it must carry the same asymmetry.

```go
type Calendar struct {
    Title          string           `yaml:"title" json:"title"`
    Header         string           `yaml:"header" json:"header,omitempty"`
    Footer         string           `yaml:"footer" json:"footer,omitempty"`
    DefaultView    string           `yaml:"default_view,omitempty" json:"default_view"` // month|week; normalized to "month" at load (RR: M4), so the wire is never tri-state
    Sources        []CalendarSource `yaml:"sources" json:"sources"`
    Event          CalendarEvent    `yaml:"event,omitempty" json:"event,omitzero"`
    WeekStart      string           `yaml:"week_start,omitempty" json:"week_start,omitempty"` // monday (default) | sunday
    EditForm       string           `yaml:"edit_form,omitempty" json:"edit_form,omitempty"`
    CreateForm     string           `yaml:"create_form,omitempty" json:"create_form,omitempty"`
    FilterControls []FilterControl  `yaml:"filter_controls,omitempty" json:"filter_controls,omitempty"`
}

// CalendarEvent is the kanban `card:` analogue — what an event chip displays.
// Omitted entirely, a chip shows the source's `summary` value alone, which is
// the sensible default for a dense grid cell.
type CalendarEvent struct {
    Fields []KanbanCardField `yaml:"fields,omitempty" json:"fields,omitempty"`
}

type CalendarSource struct {
    EntityType  string   `yaml:"entity_type" json:"entity"` // json:"entity" — SPA convention
    Where       []string `yaml:"where,omitempty" json:"where,omitempty"`
    Date        string   `yaml:"date" json:"date"`
    EndDate     string   `yaml:"end_date,omitempty" json:"end_date,omitempty"`
    Summary     string   `yaml:"summary,omitempty" json:"summary,omitempty"`
    Description string   `yaml:"description,omitempty" json:"description,omitempty"`
    Color       string   `yaml:"color,omitempty" json:"color,omitempty"`
}
```

Field names match `FeedSource` **deliberately** so one YAML anchor can feed both
— the decision recorded on the ticket. `CalendarSource` is an independent type;
no shared-type extraction, no change to `Feed`/`FeedSource`.

Divergences from `FeedSource`, each intentional: no `alarm:`/`rrule:` (export
concerns); adds `color:` (per-source visual distinction, which a merged feed
cannot express). Divergences from `Kanban`: `sources:` list instead of a single
`entity_type` (multi-type is the whole point of a calendar); no `filters:` (each
source has `where:`).

**`event:` — the `card:` analogue (added during planning).** An earlier draft of
this struct omitted it, which left "what does an event chip actually display?"
unspecified. Kanban answers this with `card: {title, fields}`; a calendar needs
the same affordance or every chip is a bare title forever.

Two deliberate differences from `KanbanCard`:
- **No `title:`.** The source's `summary:` already names the title property, and
a second way to say the same thing invites disagreement between the calendar and
the feed sharing a YAML anchor. `summary` wins; `event:` only adds *extra*
fields.
- **`event:` is on the calendar, not per source.** Sources may be different
entity types with different properties, so a shared field list cannot name
properties that exist on all of them. Resolution is per source and best-effort:
a field naming a property the source's type lacks is simply not rendered for
that source's events (it is NOT a config error — that would make multi-type
calendars nearly unconfigurable). This asymmetry with kanban's strict validation
is intentional and must be documented and tested.

`KanbanCardField` is reused as the element type — same `property`/`relation`/
`direction`/`label` shape, and inventing a structurally identical
`CalendarEventField` would be duplication for its own sake. (Contrast the
`KanbanCardField` doc at config.go:620-624, which declined to reuse
`ViewSectionField` because that type is *shared across many surfaces* and would
have leaked semantics; here the direction is the opposite — one narrow type,
already exactly right.)

**`week_start:`** — month and week grids need a week-start convention;
hardcoding Monday would be wrong for US users and hardcoding Sunday wrong for
most of Europe. Default `monday`, validated against the two values. (Check
whether `uiStore` already carries a locale to inherit from; if so, inherit and
say so.)

**Also added after review (RR: S6)** — each of these is otherwise the first
support request:
- `day_start:` / `day_end:` (default `08:00` / `20:00`) — a week grid rendering
00:00–24:00 at readable density is ~1500px tall with everything in the bottom
third. Every calendar app has this setting because every calendar app needed it.
- `max_events_per_day:` (default 4) with **in-place `+N more` expansion**. A day
with 14 events must not grow the row and break the grid. Note the obvious design
— "click through to the day view" — is blocked by scope decision 3, so expansion
must be in-place.
- `max_span:` per source (default 31d) — see §5 window-spanning.

**`color:` is an allowlisted token, not free text (RR: S7).** A raw string bound
into `:style` is unthemeable: a hardcoded `#ff0000` is unreadable in dark mode
and pins every deployment's config against future restyling. Instead `color:`
takes a named token (`blue`, `green`, `amber`, `red`, `violet`, `slate`),
validated at load and mapped to CSS custom properties — the same cross-language
allowlist idiom `ValidIconNames` + `icons.ts` already use, pinned by the same
kind of parity test.

**2. Validation (`internal/dataentryconfig/validate_calendars.go`, new)**

Modelled on `validate_feeds.go:74-175`. Per source: entity type known; `date`
required, exists, date/datetime-typed, **not `list: true`**; `end_date` same
checks plus same-kind-as-`date`; `summary` exists or type has a display
property; `description` exists; each `where` clause parses and references a real
property. Per calendar: at least one source; `default_view` in {month, week};
`edit_form`/`create_form` exist in `cfg.Forms`. Errors prefixed `calendar %q:
source[%d]`.

Note `GetValidEnumValues` (validate.go:748) — the helper kanban leans on — does
**not** apply here; the date-type check is its own predicate (there is already
`isFeedDateType` in validate_feeds.go to reuse or mirror).

**Three registration sites that are easy to miss, each a hard failure:**
- `validate.go:29` — add `"calendars": true` to `validTopLevelKeys`. **Without
it the entire config is rejected as an unknown top-level key.**
- `validate.go:48` — add `"calendar": "calendars"` to `knownTypos` (nice-to-have).
- `validate.go:~416` — nav reference check: `navigation:` naming an unknown
calendar must error, mirroring the kanban arm.

**Extraction opportunity, deliberately declined:** `validateLists` and
`validateKanbans` already duplicate the filter/filter_control loops nearly
line-for-line, and there is no shared "property exists on type" helper. A
calendar validator makes a third copy. Matching the existing shape is the
lower-risk path for this ticket; consolidating all three is a separate refactor
and is noted, not attempted here.

**3. Sidebar (`internal/dataentry/views_handler.go`) — no count**

Add `case entry.Calendar != "":` to the switch at `:461` → href
`/calendar/<name>`, icon `calendar`, and **no count**.

Decided with the user after design review (RR: S3). An unwindowed count would
read like a total — "847" beside a grid showing 12 events — true, unactionable,
and never changing as the user navigates; a badge like that teaches users to
ignore all badges. A period-scoped count is not available either: the sidebar is
rendered server-side from `/api/v1/_sidebar` and has no idea which month is on
screen, so making it period-aware would push view state into a server-rendered
nav (and brush against the §4b no-new-endpoint invariant).

`SidebarItem.Count` is already `*int` (responses.go:377-384), so "no count" is
representable with **no wire change** — the arm simply does not set it.

Consequences, both simplifications: **no `calendarCount` helper is needed**, and
the `filterCache` key-namespacing hazard (a calendar aliasing a same-named
kanban) **disappears entirely** rather than needing a test. A calendar is not an
inbox.

**4. Wire, navigation, and the long tail of registration sites**

The wire type **aliases the config type directly** (`v1.Config.Kanbans` is
`map[string]dataentryconfig.Kanban`, responses.go:260) — so the `json:` tags on
the config struct *are* the wire contract; there is no separate `v1.Calendar`.

- `config.go:~640` — `Calendar string` on `NavigationEntry`
- `internal/apiwire/v1/responses.go:~260` — `Calendars` on `v1.Config`
- `internal/dataentry/api_v1.go:1378` — `Calendars: s.Cfg.Calendars` in
`handleV1Config` (verbatim pass-through; `_config` is deliberately
principal-independent, pinned by `TestNavPermission_ConfigUnfiltered`)
- `internal/openapi/schemas.go:~420` — `"calendars": {Type: "object"}`
- `internal/lua/urls.go:26,160` — `rela.url.calendar(name, query?)`, the
name+query family (like `list`/`kanban`, not the name+entity family)
- `internal/schema/analyze.go:45` (a doc comment, no compiler help) and `:227`
— entity-type reference discovery
- `internal/schema/cleanup.go:103` **and** `:288` — **both** the plan and apply
arms. Missing either half means deleting an entity type silently leaves a
dangling calendar that fails config validation on next boot.
- `internal/dataentry/config.go:47-50` — type aliases
- `ValidIconNames` (config.go:~320) + `frontend/src/utils/icons.ts` — same commit

**Test fixtures:** ~14 files construct `Config` literals with explicit field
lists (api_v1_test.go, caldav_handler_test.go, export_test.go,
feed_handler_test.go, and others). Whether they need a `Calendars:` sibling
depends on the `omitempty` choice below.

**One deliberate deviation from kanban:** `Config.Kanbans` has **no
`omitempty`** (always serializes, `null` when nil) while `Feeds`/`EntityViews`
do. Use `omitempty` for `Calendars` — it is a new optional block, the SPA can
treat absent as empty, and it avoids touching fixtures that don't care.

**4b. Architectural invariant: no new endpoint, no new handler**

Kanban is a **pure config-projection feature** on the backend: there is no
kanban handler and no kanban data endpoint. Config ships verbatim on
`/api/v1/_config`; rows come from the **generic entity collection endpoint**
that lists use; the sidebar contributes a count. Everything else is client-side.

The calendar must preserve this. Reading rows through the existing collection
endpoint is what buys the read gate (`scopedSortedEntities`, api_v1.go:287-320),
field redaction, and neighbor visibility **for free**. Introducing a bespoke
`/api/v1/_calendars/...` data endpoint would mean re-deriving all three — the
single most important decision to get right, and the reason AC#9/#11 are cheap
rather than expensive.

(If a route were ever added, `router.go:57` and `api_v1.go:80` both carry a
standing instruction to add a probe to `router_walk_test.go`. Not needed here.)

**5. Data fetching — REWRITTEN after design review found a blocking defect**

**BLOCKER (RR: C1) — `compareValues` cannot compare datetimes. Verified.**
`internal/dataentry/helpers.go:80-107` parses both sides with the layout
`"2006-01-02"` **only**. A stored `datetime` is RFC3339. Demonstrated:

```
"2026-08-22"                parses-as-date = true
"2026-08-22T14:30:00Z"      parses-as-date = false
"2026-08-22T14:30:00+02:00" parses-as-date = false
```

So `filter[starts_at][gte]=2026-08-01` against a datetime property hits the "one
side is a date, the other isn't" arm (helpers.go:87-90) → error → `continue`
(api_v1.go:1741-1746) → **entity excluded**. A `datetime` calendar renders
**empty**, with only a per-entity `slog.Warn` as diagnosis. If both sides are
RFC3339 neither parses as a date and it falls through to **lexicographic string
comparison** (:106), which coincidentally works for same-offset `Z` values and
breaks for `+02:00` or differing precision.

An earlier draft of this plan claimed this path was verified. It was not — the
check looked at the call site's error handling, not the comparison function.
Correcting the record rather than quietly patching it.

**Fix, in scope for this ticket:** extend `compareValues` to try RFC3339 and
date-only layouts, normalizing both sides to an instant before comparing. A
bare-date bound against a datetime property means midnight (start-of-day). This
also improves `--filter`/list behaviour for every existing consumer. Add
`internal/dataentry/helpers.go` to Files-to-modify, with a table test:
date/date, datetime/datetime (`Z` vs `+02:00` at the same instant must compare
equal), datetime/bare-date, and the genuine-mismatch error.

**Half-open windows (RR: C2).** `lte=2026-08-31` against instants means "≤
2026-08-31T00:00:00" and silently drops every timed event after midnight on the
window's last day. Bounds are therefore computed as **explicit instants in the
display timezone** and sent as a half-open interval: `gte =
startOfFirstVisibleDay@tz` and **`lt` = midnight after the last visible day**
(`lt` is a supported operator, api_v1.go:1678). Half-open is the only range
shape without an off-by-one.

**Windowing is for truncation, not server load.** Filtering happens in memory
**after** `scopedSortedEntities` has loaded every entity of the type
(api_v1.go:287-320). Windowing reduces payload and client work and keeps the
result under the `listAllEntities` cap (entities.ts:59-63, BUG-5OAQUG). Store
load is unchanged from kanban. Do not advertise it as a scalability fix.

**Window-spanning events (RR: S5 — earlier answer was circular).** An event
starting before the window but ending inside it is missed by a lower bound. The
previous draft dropped `gte` for `end_date` sources and filtered client-side.
The reviewer correctly identified that as reasoning back into the very
truncation bug windowing exists to avoid — and worse, truncating oldest-first,
so the events actually on screen are the ones dropped.

**Revised approach — bound it, and make exceeding the bound visible:**
- Sources with `end_date` widen `gte` by a bounded pad, declared per source as
`max_span:` (default 31d) — the longest event the calendar promises to render.
- The pad is **not** assumed correct: after fetching, any event whose span
exceeds `max_span` triggers a visible affordance ("some long events may not be
shown"), converting a silent wrongness into a diagnosable one.
- Independently, `listAllEntities` already reports cap-hit via `meta.has_more`;
the truncation banner kanban uses (KanbanView.vue:543-545) is reused, so hitting
the cap is never silent.

This matches the project's stated preference (visible failure over silent
truncation) instead of contradicting it.

**Timezone skew:** window bounds computed in display-tz are already exact
instants, so no extra pad is needed for skew once C2's half-open instant bounds
are used.

**ACL ordering, and where `where:` is evaluated (RR: L1 — resolved)**

The review flagged the `where:`-to-query-param translation as the ACL crux
parked as a TODO. Resolved by inspection:

- `applyFilters` (helpers.go:200) is used by `views_handler.go` (sidebar counts)
and `commands.go` — **not** by the list API endpoint.
- KanbanView applies its config `filters:` **client-side**
(KanbanView.vue:138-140).

So there is **no server-side config-filter mechanism on the list endpoint** to
route through; the reviewer's preferred option (a) does not exist. `where:` is
therefore evaluated **client-side**, like kanban's `filters:`, over the
ACL-scoped rows the endpoint returns. Only the **date window** is pushed into
query params (it is a simple two-clause range that maps cleanly; `where:` is
`filter.Parse` syntax and does not).

**This preserves the feed's filter-before-redact ordering (feed_provider.go:236)
for the property that matters — existence.** The server never returns a row the
principal may not see (row-level ACL in `scopedSortedEntities`), so calendar
*membership* cannot leak hidden rows. The residual difference from the feed: a
`where:` clause naming a **field-redacted** property evaluates client-side
against the redacted value, so such an entity can drop out of the view for one
principal and not another.

That is a real, if narrow, divergence and it is **inherited from kanban, not
introduced here** — kanban's client-side `filters:` have the same property
today. Recording it explicitly rather than claiming parity the code does not
have. Mitigation: document that `where:` on a `visible:`-redacted property is
not meaningful, and cover it with AC#11's test (which asserts the selected set
is identical for a redacted and unredacted principal) — **if that test fails,
the gap is real and must be raised before implementation continues** rather than
weakened to match the behaviour.

**6. Frontend** *(revised after tracing KanbanView in detail)*

- `types/config.ts`: `CalendarConfig` / `CalendarSource` interfaces (near the
Kanban block, :296); `calendars: Record<string, CalendarConfig>` on `Config`
(:19); `calendar?: string` on `NavigationEntry` (:515).
- `stores/schema.ts`: four edits mirroring kanban — ref (:33), getter (:98),
hydration in `load()` (:287), exports (:345, :372).
- `router/index.ts`: `/calendar/:id`, lazy import, `props: true` (beside :57-62).
Note the file is `v8 ignore`d — routes are e2e-tested, not unit-tested.
- `views/CalendarView.vue` + sub-components (new).
- `utils/calendarGrid.ts` (new): **all** date math as pure functions —
`eventDay(value, kind, tz)`, `applyDayDelta(value, kind, fromDay, toDay, tz)`,
`windowBounds(view, anchor, tz, weekStart)`, plus grid geometry. Keeping these
pure turns the whole C1–C4 timezone cluster into a ~30-row table test (DST both
hemispheres, ±14h zones, month boundaries, leap day, both week starts) instead
of component tests that mount a grid to assert a date (RR: L2).
- **View and date are URL query state** (RR: M1):
`/calendar/:id?view=week&date=2026-08-22`. Query params, not route params, so
the route shape stays `/calendar/:id`. Without this, refresh loses your place
and "the week of the launch" is unlinkable — cheap now, annoying to retrofit
once bookmarks exist. `default_view` means *initial* view; an explicit `?view=`
wins.
- `utils/icons.ts`: add `calendar`. **The icon list is pinned to the Go
allowlist (`validIconNames`) by a cross-language test** — both sides must change
together or `TestIconAllowlistMatchesFrontend` fails.
- **No change** to `Sidebar.vue` (the href is minted server-side at
views_handler.go:462), `widgets/viewRouting.ts`, or `api/entities.ts`.

**Data layer must use Pinia Colada, not a bare fetch.** KanbanView wraps
`listAllEntities` in `useQuery` keyed by `entityKeys.list(type)` (:70-85) and
writes through `useMutation` with `beginOptimistic` / `rollbackOptimistic` /
`settleOptimistic` (`queries/optimisticList.ts`). That key is also what the SSE
composable invalidates (`useEvents.ts:145-149`), which is why a background
refetch never blanks the board (the spinner gates on `isPending`). A calendar
that fetched directly would lose optimistic rollback and live updates both. The
rollback does an **identity check** before reverting, so a refetch landing
mid-mutation is not stomped — reuse the helpers rather than re-implementing.

**The query key must be extended, and both halves matter (RR: S1).** Kanban
fetches everything for a type, so `entityKeys.list(type)` honestly names one
result set. A calendar fetches a **window**, so that same key would name
different result sets (August, September, week-of-3rd) depending on when it was
populated — navigate month→next→back and you serve stale data under a matching
key.

But simply namespacing the key **silently opts out of the SSE invalidation**
that `useEvents.ts:145-149` performs on the un-namespaced key, killing live
updates. Both halves must be solved together:

- Extend the key factory so the calendar's key is a **descendant** of
`entityKeys.list(type)` — e.g. `[...entityKeys.list(type), {window, tz, where}]`
— so the existing prefix invalidation still matches it.
- **Verify Pinia Colada's invalidation is prefix-matching** at the version in
`package.json` before relying on it; if it is not, add the calendar key to
`useEvents.ts` explicitly.
- `effectiveTimezone` belongs in the key (see §7 day-assignment): changing
display timezone changes the window.

This makes `frontend/src/queries/entities.ts` (the key factory) a modified file
— an earlier draft wrongly listed the api layer as unchanged.

**Component size is a lint constraint.** ESLint warns at `max-lines: 500` for
Vue files ("catches god components"); KanbanView is 1043 and already over.
CalendarView must be split from the start — grid/day-cell/event-chip as
sub-components, date math in `calendarGrid.ts`.

**Dense-surface widget rules apply.** Event chips showing property values must
route through `densePropertyRoutingHint` + `defaultRegistry.resolveFromHint`,
resolved **once per configured field, not per event** (the
200-warnings-per-render problem, RR-UD2A). Empty renders blank, never a
placeholder. Per frontend/CLAUDE.md: "Do not add a per-type `v-if` to a view."

**Which day does a timed event belong to? (the subtlest correctness question)**

A `datetime` value is stored as a UTC instant. Which grid cell it occupies
depends on the *display* timezone: `2026-03-01T00:30:00Z` is 1 March in UTC but
**28 February** in `America/New_York`. Rules for this implementation:

- **Day assignment is computed in `uiStore.effectiveTimezone`** (ui.ts:76), the
same zone `formatDatetime` renders in. Anything else puts an event in a cell
whose printed time contradicts its position.
- **All-day (`date`) values are calendar dates, never instants.** They must not
be converted through a timezone at all — `2026-03-01` is 1 March everywhere.
Mixing the two conversions is the classic off-by-one-day calendar bug, which is
exactly why `parseDate` (format.ts:21-35) parses bare dates as local rather than
via `new Date(str)` (which parses as UTC and shifts the day for negative
offsets).
- **DST:** a grid must be built by incrementing calendar dates, never by adding
24h in milliseconds — a DST day is 23 or 25 hours. Week view's hour axis has a
missing or doubled hour on transition days; acceptable to render naively for
month+week, but must not shift day boundaries.

**Stated invariant (RR: C3).** Without an explicit rule the implementer will
reach for `new Date(iso).getDate()` and silently get the *browser's* zone, which
disagrees with `uiStore.effectiveTimezone` for any user who set a display
timezone. The event then renders on the wrong day, the drag delta is computed
from that wrong day, and **the drag writes a wrong value** — a corruption path,
not a display bug. So:

> **Day assignment.** A `datetime` event's grid day is the calendar date of
> `TZDate(instant, uiStore.effectiveTimezone)`. A `date` event's grid day is the
> literal `YYYY-MM-DD`, timezone-independent (via the exported `parseDate`).
> The two never mix within a source (enforced by the same-kind validation).

Pinned by a test rendering the same instant under `Pacific/Kiritimati` (+14) and
`Pacific/Niue` (−11) and asserting different cells.

**Consequence for the query key:** the window is computed in display-tz, so
`effectiveTimezone` is an input to the fetch. Changing timezone must refetch —
see the query-key note in §6.

**Date handling — three specifics found in `utils/format.ts`:**
- `parseDate` (:21-35) parses bare `YYYY-MM-DD` in **local** time deliberately,
so `2024-01-15` is Jan 15 in every zone. It is currently **private**; the grid
must use it (export it) rather than `new Date(str)`, which would parse as UTC
and shift the day for negative offsets.
- `localInputToUtcISO(local, tz)` (:89-105) is what a `datetime` drag patch must
use to build the stored value; `uiStore.effectiveTimezone` (ui.ts:76) is the
timezone source of truth.
- **No `date-fns`** in the dependency tree — only `@date-fns/tz`
(package.json:23). Month/week arithmetic is hand-rolled (as assumed in
Research); adding `date-fns` would be a new dependency and is not proposed.

**Nearest structural precedent for the month grid** is the kanban *swimlane*
board: `swimlaneGridStyle` (:250-255) plus the 2D cell markup (:618-697), where
each cell is already a `@dragover`/`@drop` target holding a list of cards. That
maps 1:1 onto a day cell holding events.

**7. Drag write path — two corrections found during planning**

*(a) There is no read-only property concept.* `PropertyDef`
(metamodel/types.go:284-312) has `Type`, `Required`, `List`, `Unique`, `Format`
— no `readonly`/`computed`. The original AC#8 was wrong. The real load-time
failure modes are: property absent, wrong type, `list: true`. Non-updatable is
an **ACL/runtime** matter, handled by `actionAllowed`, not config validation.

*(b) Dates must be written in the property's declared format.*
`PropertyDef.Format` is a per-property Go layout defaulting to `2006-01-02`
(metamodel/types.go:291,483). Writing a hardcoded ISO date would fail validation
in any project using a custom format. The drag handler must format using the
property's declared format, which means the SPA needs that format — check
whether the schema endpoint already exposes it; if not, exposing it is part of
this ticket.

*(c) All-day and timed drags write different things.* Dropping on a day cell is
a **whole-day delta** in both cases, but the serialization differs:
- `date` source → format the new calendar date with the property's declared
`Format` (default `2006-01-02`). No timezone conversion.
- `datetime` source → **preserve the wall-clock time of day** and move only the
date, then serialize with `localInputToUtcISO(local, effectiveTimezone)`
(format.ts:89-105). Naively adding 24h × N to the UTC instant shifts the
displayed time by an hour across a DST boundary — a 09:00 meeting becomes 08:00.
Month/week drag changes the day, never the time.

Drag applies a delta: `newDate = oldDate + (dropDay - originalDay)` in whole
days, and when `end_date` is set the same delta moves it too, preserving
duration. A single `updateEntity` patch carries both properties (one write, no
torn intermediate state where start has moved and end has not). Optimistic
update + rollback on failure via the Colada helpers, mirroring KanbanView.

**(d) DST-safe delta (RR: C4).** `newDate = oldDate + N*86400000ms` is wrong
across a DST boundary — dragging across spring-forward shifts a 09:00 meeting to
08:00, and repeated drags drift. The delta must be applied in **calendar terms
in the display timezone**: decompose to (y, m, d, h, min) in
`effectiveTimezone`, replace the date part with the drop day, recompose via
TZDate → UTC. That is exactly the `localInputToUtcISO` round-trip
(format.ts:89-105), and it inherits that function's documented spring-forward
normalization — which is the desired behaviour, recorded here so nobody later
"fixes" it.

**AC#6 wording is corrected as a result.** Across a DST boundary, *preserving
duration* and *preserving wall-clock start+end* are different operations. A
human expects "my 09:00–10:00 meeting stays 09:00–10:00", so the rule is
**wall-clock preservation**, and AC#6's "duration is preserved" is reworded
accordingly.

**(e) Drag is day-granular in v1.** Dragging changes an event's **day** and
preserves its **time-of-day**. Vertical position in week view does NOT set the
time. This is a defensible v1 scope, but it contradicts the natural expectation
in a week grid, so it is stated in the ACs rather than left implicit.

**Format is a Go layout, and that is a trap (RR: S4).** `PropertyDef.Format` is
a **Go time layout** (`"2006-01-02"`, metamodel/types.go:291) and it *is*
already on the wire (`frontend/src/types/schema.ts:30` has `format?: string`).
But JavaScript has no Go-layout formatter — honouring `format: "02/01/2006"`
would require writing a Go-layout interpreter in TS, with its own bug surface.
Treating this as a one-line lookup is how an `l` ticket becomes `xl`.

**Scoped down:** validate at config load that a calendar's `date`/`end_date`
property uses a **supported format** — the default (`2006-01-02`) or RFC3339 —
and fail the load with a clear message otherwise. Custom-format calendars are a
documented follow-up. This is consistent with decision 4 (fail at load, never
break on first drag) and it is honest about what v1 supports.

**Alternatives rejected:**
- *Shared `EventSource` type with feeds* — rejected on the ticket: refactors a
shipped config with a live CalDAV consumer to buy consistency; YAML anchors give
reuse for free.
- *Render `feeds:` directly as a view* — no home for `default_view`,
`edit_form`, `filter_controls`; couples an interactive surface to an export one.
- *Calendar library* — see Research.
- *Reuse `internal/calfeed.Event` as the SPA's event model* — rejected: it is
RFC 5545-shaped and lossy (no entity identity/type), which is exactly what the
view needs. The view carries entities, not events.

**Files to modify:**

*Go (new):* `internal/dataentryconfig/validate_calendars.go`,
`internal/dataentryconfig/validate_calendars_test.go`

*Go (edit):* `config.go` (Calendar/CalendarSource, Config.Calendars,
NavigationEntry.Calendar), `validate.go` (:339 wiring), `views_handler.go`
(sidebar case + calendarCount), `internal/apiwire/v1/responses.go`,
`internal/openapi/schemas.go`, `internal/lua/urls.go`,
`internal/schema/analyze.go`, `internal/schema/cleanup.go`,
`internal/dataentry/acl_sidebar_test.go`

*Frontend (new):* `views/CalendarView.vue` + sub-components (grid, day cell,
event chip — to stay near the 500-line lint budget), `utils/calendarGrid.ts`,
`utils/calendarGrid.test.ts`, `views/CalendarView.test.ts`,
`views/CalendarView.drag.test.ts`, `views/CalendarView.pagination.test.ts`

*Frontend (edit):* `types/config.ts` (:19, :296, :515), `router/index.ts`,
`stores/schema.ts` (:33, :98, :287, :345, :372), `utils/icons.ts` (+ Go
`validIconNames`, pinned by test), `utils/format.ts` (export `parseDate`),
`utils/icons.test.ts`, `types/config.test.ts`, `stores/schema.test.ts`

*Not modified (verified):* `Sidebar.vue`, `widgets/viewRouting.ts`.
`api/entities.ts` is reused as-is, but `queries/entities.ts` (the key factory)
IS modified — see §6.

*Docs:* `docs/data-entry.md` (calendars section + anchor note),
`docs/acl-security.md` if the gating table enumerates view kinds

## Security Considerations

- [x] Input sources identified
- [x] Input validation approach defined
- [x] Security-sensitive operations identified
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**
- *`data-entry.yaml` calendars block* — operator-authored, trusted-ish but
validated at load against the metamodel (allowlist: property must exist and be
of an accepted type). Invalid → config load fails with a message naming the
calendar and the source index. Per CLAUDE.md config names are **not** secret,
so errors may name them.
- *Date-range query params* — client-supplied; parsed as dates server-side by the
existing filter pipeline. Malformed → existing `errBadFilter` 400 path.
- *Calendar id in the URL* — used to look up a config key; unknown → 404. No
path construction from it.
- *Drag payload* — a normal `PATCH` through the existing entity write path;
entitymanager validation and ACL apply unchanged. The SPA gate is UX, not
enforcement.

**Security-Sensitive Operations:**
- *Row-level read gate*: events come from the ACL-scoped list endpoint; a hidden
entity is never returned, so it cannot appear as an event (AC#9).
- *Field redaction ordering*: filter-on-raw, redact-on-render
(feed_provider.go:236). Getting this backwards makes membership leak information
about hidden field values.
- *Write*: `actionAllowed(entity,'update')` gates the UI; the server re-checks.
No new write path is introduced — `updateEntity` only.
- *No new endpoint* is added: the calendar reuses the list endpoint, so it
inherits its ACL, rate and pagination behaviour rather than re-deriving them.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified
- [x] Negative test cases defined
- [x] Integration test approach defined

**Test Scenarios (AC → test):**

| AC | Test |
|----|------|
| 1 | Go: sidebar handler emits `/calendar/x` entry. Frontend: route resolves to CalendarView. |
| 2 | `calendarGrid.test.ts`: month grid for a known month has 5–6 weeks, correct leading/trailing days, correct today index. Week grid = 7 days from configured week start. |
| 3 | CalendarView.test.ts: two sources over different types both render; events merged and ordered. |
| 4 | Grid test: `date` source → all-day chip; `datetime` source → timed, ordered by time. |
| 5 | Click event → router push to entity; with `edit_form` → form opens. |
| 6 | `CalendarView.drag.test.ts`: drop on a later day issues one `updateEntity` with the new date; with `end_date`, both move by the same delta (duration preserved). |
| 7 | Drag test: entity with `update` not allowed → `draggable=false`, no write on forced drop event (the defence-in-depth KanbanView.vue:461-466 already models). |
| 8 | `validate_calendars_test.go` table: missing property, wrong type, `list: true`, kind mismatch, unknown entity type, bad `where`, bad `default_view`, unknown `edit_form` — each produces a specific error naming calendar + source index. Plus: `calendars:` accepted as a top-level key, and `navigation:` naming an unknown calendar errors. |
| 9 | Go integration: a principal denied a type sees no events from it. Extend `acl_sidebar_test.go` (:60-80 `installSidebarConfig` publishes a kanban; add a calendar) and assert the count is the visible subset, mirroring `TestACLSidebar_CountsMatchList` (:109). |
| 10 | Sidebar test: nav entry with unheld `permission:` is absent (`permitsNavEntry`, views_handler.go:353). |
| 11 | Go: `where:` on a `visible:`-redacted property selects the same set for a redacted and unredacted principal. **If this fails, stop — see §5.** |
| 12 | `CalendarView.test.ts`: a `datetime` source renders events (regression guard for the `compareValues` fix); Go table test in `helpers_test.go` for the extended comparison. |
| 13 | Window-edge test: timed event at 23:30 on the last visible day renders (guards the half-open bound). |
| 14 | Mount with `effectiveTimezone` = `Pacific/Kiritimati` (+14) and `Pacific/Niue` (−11) over one instant; assert different cells. |
| 15 | Navigate month→next→prev; assert each period's data is correct and not served from a stale key. |

**Edge Cases:**
- Month with 6 week-rows (e.g. May 2026, Aug 2026); Feb in a leap year
- DST transition inside the rendered week (a "day" of 23/25h) — grid must be
built from calendar dates, not by adding 24h repeatedly
- Entity whose date property is empty → skipped, not an error (feed rule)
- Entity whose date fails to parse → skipped; must not blank the grid
- `end_date` before `date` → render as single-day; do not produce a negative span
- Event spanning the window boundary: starts before the window, ends inside it (§5) — and the mirror case, starts inside and ends after
- Zero events in range → empty grid, not a spinner or error
- Many events on one day → overflow indicator, not unbounded growth
- Week-start convention (Mon vs Sun) — configurable, both pinned in a test
- Drag across a DST boundary preserves wall-clock time (spring-forward and
fall-back, northern and southern hemisphere)
- Leap day 2028-02-29
- Multi-source merge ordering is deterministic: sort by (start instant, source
index, entity id). Without the tiebreakers, all-day events from two sources on
the same day reshuffle between renders (RR: M2)
- A day with more events than `max_events_per_day` shows `+N more` and expands
in place
- Empty state (no events in range) is visually distinct from loading (RR: M3)
- An `event:` field naming a property one source's type lacks: not rendered for
that source, not a config error
- A source whose type the principal cannot read → contributes nothing, no error

**Negative Tests:**
- Every validation branch above fails **config load** with a message naming
calendar and source index (fail-closed: a bad calendar stops startup rather than
rendering a subtly wrong board)
- Unknown calendar id → 404
- Drag rejected by the server (422/403) → optimistic update rolls back and the
error surfaces (no silent failure — a project rule)

**Integration approach:** Go handler tests exercise sidebar + ACL against a real
config and store, as `acl_sidebar_test.go` does.

Frontend uses **vitest + @vue/test-utils with `mount` (never `shallowMount`)** —
assertions are on rendered widget output. Per-test `createPinia()` +
`PiniaColada` plugins; schema seeded by writing into the store maps
(`schemaStore.calendars.set(id, {...} as never)`);
`_setEntityPluralForTest(type, plural)` in `beforeEach` is mandatory or every
request throws; `vue-router` mocked wholesale.

**Two distinct mock seams, and the choice matters:** `vi.mock('@/api')` for
behaviour tests, but paging must be tested at the HTTP-client seam
(`vi.mock('@/api/client')`) — mocking `@/api` cannot intercept
`listAllEntities`' module-internal call to `listEntities`, so only the client
seam exercises the real loop inside a mounted component (RR-5YVXMK).

**Drag is not unit-tested for kanban today** (it is e2e territory). This ticket
adds `CalendarView.drag.test.ts` regardless — drag *is* the write path here, and
the delta/duration arithmetic must be pinned by a unit test rather than left to
e2e. E2E lives in `/e2e/` and runs against the built binary (`just e2e`).

Pure date math is tested without mounting via `calendarGrid.ts`.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed
- [x] Effort estimated

**Risks:**

| Risk | Mitigation |
|------|-----------|
| Date/timezone bugs (DST, month boundaries, UTC-vs-local drift) — the classic source of calendar defects | Isolate all date math in `calendarGrid.ts` as pure functions; table tests over known-tricky months incl. DST and leap year. Treat all-day dates as calendar dates, never instants. |
| Window-spanning events missed by a naive range filter | **Resolved in §5**: sources declaring `end_date` drop the lower bound and filter client-side; pinned by a test |
| Per-property date `Format` breaks drag writes in projects with a custom format | §7(b): format via the declared layout; verify the SPA can see it, expose it if not |
| CalendarView.vue accreting like KanbanView (1043 lines, already over the `max-lines: 500` ESLint warning) | Split into sub-components from the start; date math lives in `calendarGrid.ts`, not the component |
| Losing optimistic rollback / live SSE updates by fetching outside Pinia Colada | Reuse `useQuery` + `beginOptimistic`/`rollback`/`settle` and the `entityKeys.list(type)` key that `useEvents` invalidates |
| Icon allowlist drift between Go and TS | Add `calendar` to both in the same commit; a cross-language test pins them |
| Config surface disagreement (field names/shape) surfacing at review | The shape is settled on the ticket and mirrors two existing precedents; `/design-review` before implementation |
| Missing one of the ~13 registration sites (esp. `validTopLevelKeys`, or only one of cleanup.go's plan/apply arms) — failures are delayed and confusing | Enumerated explicitly in Approach §4 as an implementation checklist; the cleanup.go plan/apply pair gets a test |
| Introducing a calendar-specific data endpoint and losing the ACL pipeline | Stated as an invariant in §4b: rows come from the generic collection endpoint |
| **`compareValues` datetime defect (§5)** — would make every `datetime` calendar render empty with green CI | Fixed in this ticket with a table test; AC#12 is the regression guard. **Verify the fix before writing frontend fetch code.** |
| Timezone/DST correctness generally — the cluster the review found (C1–C4) | All date math is pure functions in `calendarGrid.ts` (`eventDay`, `applyDayDelta`, `windowBounds`) tested as a table across DST, ±14h zones, month/leap boundaries — not via mounted components (RR: L2) |
| Go-layout `Format` requiring a TS interpreter (scope blowout) | v1 validates for supported formats at load and fails clearly; custom formats deferred (§7) |
| Scope creep toward day/year or ICS publishing | Both already split into TKT-S18C1U and a noted follow-up |

**Effort:** `l`, but at the top of that band after review. The design review
added real work that was not in the original estimate: the `compareValues`
datetime fix with its table test (§5), the timezone/DST cluster as pure
table-tested functions (§7, L2), the query-key extension plus SSE prefix
verification (§6), and four config additions (`event:`, `week_start`,
`day_start`/`day_end`, `max_events_per_day`, `max_span`).

Not raised to `xl` because each addition is small and well-specified, and the
`Format` scope-down (§7) removed the one genuinely open-ended item (a Go-layout
interpreter in TS). If the `compareValues` fix turns out to have wider blast
radius across existing `--filter` consumers than expected, revisit.

## Documentation Planning

- [x] User-facing docs identified
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**
- [x] `docs/data-entry.md` — new `calendars:` section (config table, examples,
month/week behaviour, drag semantics), plus a note that a source list can be
shared with `feeds:` via a YAML anchor (the guide itself is TKT-6RSPA2)
- [x] `docs/acl-security.md` — only if it enumerates view kinds for gating
- [ ] `docs/metamodel.md` — N/A (no metamodel change)
- [ ] `docs/cli-reference.md` — N/A (no CLI change)
- [ ] `CLAUDE.md` — N/A (no new cross-cutting pattern)

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan (one open question below)

**Design Review Findings:** Reviewed via `cranky-code-reviewer`. Findings were
addressed **in the plan** rather than filed as `review-response` entities, since
no code exists yet and every finding changed the design. Disposition:

| # | Finding | Disposition |
|---|---------|-------------|
| C1 | `compareValues` cannot compare datetimes → datetime calendars render empty | **Confirmed by execution.** Fix scoped into this ticket (§5); AC#12 guards it. The plan's earlier "verified" claim was wrong and is corrected in place. |
| S3 | Unwindowed sidebar count is misleading | **Decided with the user: no count for calendars** (§3). Also removes `calendarCount` and the cache-key aliasing hazard. |
| C2 | `lte` truncates the window's last day for timed events | Accepted — half-open instant bounds with `lt` (§5) |
| C3 | Day-assignment for timed events undefined | Accepted — invariant stated + ±14h test (§7) |
| C4 | Day-delta in ms is wrong across DST | Accepted — calendar-term delta via TZDate round-trip; AC#6 reworded (§7) |
| S1 | Query key wrong for a windowed fetch; namespacing breaks SSE | Accepted — descendant key + verify prefix invalidation (§6) |
| S2 | No event-chip config | Accepted — `event: {fields}` added (§1). **Decoder concern tested and cleared:** strict key checking is top-level only, so a YAML anchor carrying calendar-only keys parses fine into `FeedSource` — decision 1's mechanism works. |
| S4 | `Format` is a Go layout; no TS formatter exists | Accepted — v1 validates for supported formats and fails at load (§7) |
| S5 | Dropping `gte` reasons back into the truncation bug | Accepted — the earlier answer was circular; replaced with bounded `max_span` + visible affordance (§5) |
| S6 | Missing week_start / day range / overflow config | Accepted — all added (§1) |
| S7 | `color:` free text is unthemeable | Accepted — allowlisted token (§1) |
| M1 | No URL state for view/date | Accepted — query params (§6) |
| M2 | Multi-source merge ordering nondeterministic | Accepted — explicit sort with tiebreakers (edge cases) |
| M3 | Empty state unspecified | Accepted (edge cases) |
| M4 | `default_view` tri-state on the wire | Accepted — normalized in Go (§1) |
| M5 | AC#6 wording wrong | Accepted — reworded |
| L1 | `where:` translation parked as a TODO | **Resolved by inspection** (§5). The reviewer's preferred option (a) does not exist — there is no server-side config-filter path on the list endpoint. `where:` is client-side like kanban's `filters:`; the inherited redaction divergence is documented rather than papered over. |
| L2 | Make date math a pure, table-tested module | Accepted (§6) |
| L3 | Test-plan gaps | Accepted — ACs #12–15 and edge cases added |
