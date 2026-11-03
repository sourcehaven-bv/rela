---
id: PLAN-6N8TTI
type: planning-checklist
title: 'Planning: Busy column on list rows'
started: "2026-10-30"
completed: "2026-10-30"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Problem.** TKT-21G6JS added `?busy=<channel type>` on `GET /api/v1/{plural}`
and `_busy` on each row. Only a kanban card asks for it, through a `busy:` card
field. TKT-W5ZSYJ made a busy list subscription wake live on session moves.
Neither ticket is tied to the kanban: the list endpoint, the row extra and the
live wake serve any list request that carries `busy`. What is missing is a way
to ask for it from a list view and to show it there. `GUIDE-realtime.md` says so
today: "A list view has no busy column yet (TKT-TU3XH4)."

**How it works today (from the code).**

- *List columns.* `dataentryconfig.ListColumn`
(`internal/dataentryconfig/config.go:961`) is one column of a list. It sets
`property`, `relation` or `face: true`, plus `label`, `sortable`, `link` and
`direction`. The same type is reused by dashboard table cards
(`DashboardCard.Columns`) and view sections (`ViewSection.Columns`,
`parent_columns`, `child_columns`, `columns_by_type`).
- *The face column* is the closest precedent. It is a column that shows
row state rather than a property. `validateLists`
(`internal/dataentryconfig/validate.go:1247`) refuses it with `property` or
`relation`. In the SPA, `columnKey` returns `face`, `columnHeader` falls back to
`t('list.column.face')`, it is not sortable (no `property`), and
`EntityList.vue` marks it `primary`, so it survives the compressed layout. The
export labels it `Face`.
- *The kanban busy field.* `KanbanCardField.Busy` is checked in three
layers. `checkCardFieldCounts` and the `busySeen` loop in `validateKanbans`
check the shape: it stands alone, and a card has at most one.
`refuseKanbanOnlyFields` refuses it on calendars and gantts. `ValidateChannels`
(`internal/dataentryconfig/channels.go:228`) checks the channel type against
`realtime.yaml` through `busyFields` and `busyFieldErrors`.
`HasRealtimePresentation` uses `busyFields` too, so every caller of it checks
busy fields: `rela validate` (`internal/cli/validate.go`), Configure
(`cmd/rela-server/configure.go` through `projectsetup.CheckChannels`), boot
(`SetRealtime`) and reload (`checkChannelsPresentation`).
- *The board request.* `KanbanView.vue` takes `busyType` from the first card
field with `busy` and sets `params.busy`. `resolvedCardFields` keeps the `_busy`
entries of that type and renders `RlBusyIndicator` through `CardFieldList.vue`.
- *The list request.* `EntityList.vue` builds `queryParams` from
`listBaseParams` and the user's state. The same params feed the list query and
the live path (`liveListPath`). A paged list subscribes its own page. A grouped
list subscribes `page=1&per_page=100`.
- *The live wake.* `serveRowBusy` (`internal/dataentry/busy_rows.go`)
calls `recordBusy` with the served rows, for any list request with `busy`.
`liveListKeys` (`internal/dataentry/livepath.go:71`) already accepts `busy`. The
wake holds the anchors of the rows on the page.
- *Configure.* The allowlist (`internal/configedit/allowlist.go`) lists
each editable list column key by name (`lists.*.columns.[].property` and so on).
`lists.*.columns.[].face` is locked. `TestAllowlist_ClassifiesEveryKey` fails on
a new config key that is in neither list. Kanban busy fields are editable
through `kanbans.*.card.**`.

**Terms used below.**

- *Busy column*: a list column with `busy: <channel type>`.
- *Busy type*: the channel type the busy column names.

**Scope (in):**

- `data-entry.yaml`: a list column entry `busy: <channel type>` in
`lists.<id>.columns`.
- Load checks for the column's shape, and for its channel type against
`realtime.yaml`, at boot, on reload, in `rela validate` and in Configure.
- Refusal of `busy` on dashboard card columns and view section columns.
- Configure allowlist: `lists.*.columns.[].busy` editable, the dashboard
keys locked.
- SPA: `EntityList` sends `busy` when the list has a busy column, and the
column renders `RlBusyIndicator`. Header label with i18n, no sorting, and a
place in the compressed layout.
- The list export leaves the busy column out.
- The Configure list page handles a busy column: it shows the channel type
as the source, offers no Sortable switch, refuses a move or removal that would
make the busy column first, and the change review titles it.
- A grouped list keeps its busy column when the title column shows the
grouped property (a fix to the existing column filter).
- One `RlTable` story with a busy column. No component change.
- Docs: `GUIDE-data-entry.md`, `GUIDE-realtime.md` (regenerated into
`docs/data-entry.md` and `docs/realtime.md`), and one sentence in
`docs/data-entry/api-reference.md`, which is hand-written.

**Scope (out):**

- Any server change to `?busy=`, `_busy`, `BusyReader`, the live wake or
the safety replay. They serve lists as they are.
- Sorting or filtering by busy state. See "Sorting" below.
- A Configure form control to add a busy column. The allowlist admits the
key, so a raw edit works, and the list page shows and keeps the column.
- Busy columns on dashboard cards, view sections and the sidebar flyout.
- A list-level `busy:` key. See Alternatives.
- Watching rows past the first 100 of a grouped list. This is the same
limit the board has.
- Live busy updates on a list with a search term. The server refuses `q`
on a live path, so such a list has no subscription today. The docs say so.
- Cutting the busy read of a large grouped list. See Risks.

**Acceptance Criteria:**

1. **The column loads.** A ticket list with `columns: [{property: title},
{busy: run}]` loads. A busy column may set `label`. Refused at `list "<id>":
column[i]`:
   - `busy` with `property`, `relation`, `direction`, `face`, `link` or
`sortable: true` ("busy cannot be combined with property, relation, direction,
face, link or sortable");
   - a second busy column in one list ("a list has at most one busy
column");
   - a busy column at `column[0]` ("the first column is the row title and
cannot be busy").

Test: `internal/dataentryconfig/validate_test.go`
`TestValidateConfig_ListBusyColumn` (table: alone, with label, each combined
key, twice, first column).
2. **Lists only.** `busy` on a column of a dashboard card (`dashboard` and
`dashboards.<id>`, query and list cards, any display) or a view section
(`columns`, `parent_columns`, `child_columns`, `columns_by_type`) is refused
with "busy is only supported on list columns". A section whose source does not
resolve is still checked, and each busy column is reported once. Test:
`TestValidateConfig_BusyColumnOnlyOnLists` in `validate_test.go` (one row per
surface, plus a query card, a list card with `display: list`, a section with an
unresolvable source, and a `columns_by_type` section that asserts exactly one
error).
3. **The channel type is checked against `realtime.yaml`.** Through
`ValidateChannels`, with the `list "<id>": column[i]` prefix: an undeclared
type, a type without a session, a session with no working state, and a type that
does not attach to the list's entity type are errors. A type whose initial state
reads working by default is a warning. With realtime off, a busy column is the
warning "realtime is off (no realtime.yaml), so this busy column shows nothing".
The kanban text ("this busy field shows nothing") is unchanged. A config whose
only realtime item is a list busy column makes `HasRealtimePresentation` true.
Test: `internal/dataentryconfig/channels_test.go`
`TestValidateChannels_BusyListColumn` (table, no `channels:` block in any case,
and one `HasRealtimePresentation` assertion).
4. **Every check path sees it.** Without a `channels:` block and without a
kanban busy field:
- `rela validate` rejects a list column `busy: nope`.
Test: `internal/cli/validate_channels_test.go`
`TestValidate_BusyListColumnWithoutChannels`.
- `projectsetup.CheckChannels` returns the error.
Test: a case in `internal/projectsetup/validate_test.go` `TestCheckChannels`.
- A Configure save with `busy: nope` on a list column is refused.
Test: a list case in `cmd/rela-server/configure_test.go`
`TestConfigure_BusyFieldAgainstRealtime`.
- `SetRealtime` refuses `busy: nope` and, with realtime off, logs the
warning for `busy: run`. Test: `internal/dataentry/realtime_wiring_test.go`
`TestSetRealtime_BusyListColumn`.
- A `data-entry.yaml` reload with `busy: nope` on a list column is
refused and the last good config stays. A reload with `busy: run` is applied.
Test: a list case in `realtime_wiring_test.go`
`TestReloadConfig_ChecksBusyField`.
5. **Configure allows the key on lists only.** A Configure edit that sets
`lists.<id>.columns[i].busy` passes `CheckEdit`. One that sets it on a
`dashboard` or `dashboards.<id>` card column is refused as locked. Test: cases
in `internal/configedit/allowlist_test.go` `TestCheckEdit`;
`TestAllowlist_ClassifiesEveryKey` passes with the new field.
6. **The list asks for busy state.** A list with a busy column sends
`busy=run` on its list request and on its live subscription path. A grouped list
sends it on every page it loads. A list with a search term still sends `busy` on
its list request (it has no live path). A list without a busy column sends no
`busy`. Test: `frontend/src/components/lists/EntityList.busy.test.ts` cases
"sends busy for a busy column", "no busy without a busy column", "live path
carries busy", "grouped list sends busy on each page", "searched list still
sends busy".
7. **The cell shows the indicator.** A row whose `_busy` has one entry of
the busy type shows `RlBusyIndicator` in the busy column, with the accessible
name "Agent run: Planning, for 3 hours". Two entries show the first and "+1",
and the name lists both. A row with `[]`, without `_busy`, or with entries of
another type shows an empty cell. The header reads the column's `label`, else
"Busy" (en) or "Bezig" (nl). The header has no sort control. The column is
`primary`, so it stays in the compressed layout. There an empty busy cell keeps
its slot, as any empty primary cell does, so the badges in each row stay
aligned. `isEmpty` is true for a row with no entry; it matters only to the
stacked layout. The busy column has a fixed width of 160 px. Test:
`EntityList.busy.test.ts` cases "indicator in the cell", "+1 for two entries",
"empty for [] and absent", "ignores another type", "header label and i18n",
"primary and isEmpty"; `frontend/src/components/lists/tableColumns.test.ts`
cases for `columnKey` (`busy`), `columnHeader` (label, then `list.column.busy`),
`sortable: false` and `width: 160`, and a collision case (a face column and a
busy column in one list keep distinct keys).
8. **A paged list wakes for its own page.** A live subscription on
`/api/v1/tickets?list_id=all&page=2&per_page=2&busy=run` records a wake with the
page-2 rows only. A session move on a page-2 row replays it and frames it. A
move on a page-1 row does not select it. Test:
`internal/dataentry/live_busy_test.go` `TestLive_BusyListPagedWake`. It uses the
manual ticker and a positive control in the same round, as the TKT-W5ZSYJ review
asked of the negative live tests.
9. **The list view's own row set bounds the busy read.** A request with
`list_id` whose list scope or filters drop a row passes only the served rows to
the busy read. The dropped row is in neither the response nor the reader's
anchors. Test: `internal/dataentry/busy_rows_test.go`
`TestListBusy_ListIDScoped`.
10. **The export leaves it out.** A list export of a list with a busy
column has no busy column in its header or rows. Test:
`internal/dataentry/export_list_busy_test.go` `TestExportList_SkipsBusyColumn`.
11. **Configure handles the column.**
    - The list page shows a busy column with the title `run` (its label
when set). Its source text and header placeholder read `busy: run`.
    - A busy or face column has no Sortable switch. The preview marks it
not sortable.
    - A move that would put a busy column first is not applied, and so is
the removal of column 0 while column 1 is busy. The Remove button is disabled in
that case. The Columns section says why: "A busy column cannot be the first
column."
    - A save that reorders or relabels columns keeps `busy`.
    - The change review titles an added or removed busy column `run`, not
`column`.

Test: `frontend/src/configure/__tests__/models.test.ts` cases "busy column model
and title", "column source text" and "busy column cannot become first" (the move
and remove guards as pure helpers); `changes.test.ts` case "busy column title in
the review"; `frontend/src/views/configure/ListView.test.ts` (new) cases "busy
row has no Sortable switch and shows busy: run" and "move to the top is
refused".
12. **Library story.** `RlTable.stories.ts` gains a story "BusyColumn": one
busy cell, one empty cell and one with "+1", at 160 px, in the normal and the
compressed layout. In the compressed layout the empty cell keeps its slot. Test:
the story renders in the Storybook smoke run.
13. **Docs.** `GUIDE-data-entry.md` "Column Options" documents `busy` with
an example and the rules. `GUIDE-realtime.md` "Busy indicator" covers boards and
lists, and the line "A list view has no busy column yet" is gone. "When it
updates" names the grouped-list limit and that a searched list does not update
live. `docs/data-entry/api-reference.md` says that a list's busy column sends
`busy` too. Test: reviewed; `just docs` regenerates `docs/`. The "Busy column
example" block in `docs/data-entry.md` is pinned the way
`docsExampleChannelsDisplay` in `channels_test.go` pins the channels block:
`TestDocsExample_BusyListColumn` reads the block, parses it, and runs
`ValidateConfig` and `ValidateChannels` against a fake schema and realtime
config.
14. **Grouping keeps the busy column.** A list grouped by the property its
first column shows, with the busy column second, still renders the busy column.
The title column is taken off before the grouped-property column is filtered
out, so the busy column is never sliced away as the title. Test:
`EntityList.busy.test.ts` case "grouped by the first column keeps the busy
column".

## Research

- [x] ~~For larger features: run `/research` to create a structured research doc~~ (N/A: the server side exists; this ticket reuses the face column and the kanban busy field)
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A

**Existing Solutions:**

- Libraries: none needed. No new Go or npm package.
- Codebase patterns reused:
  - The face column: `ListColumn.Face`, its check in `validateLists`
(`validate.go:1247`), `columnKey` and `columnHeader` in
`frontend/src/components/lists/tableColumns.ts`, `primary` in `EntityList.vue`
(`tableColumns` computed), `list.column.face` in
`src/i18n/locales/{en,nl}.json`, `columnLabel` in
`internal/dataentry/export_list.go:564`, and the locked
`lists.*.columns.[].face` in the Configure allowlist.
  - The kanban busy field: `checkCardFieldCounts`, the `busySeen` loop,
`refuseKanbanOnlyFields`, `busyFields`, `busyFieldErrors` and
`HasRealtimePresentation` (`internal/dataentryconfig`).
  - The board request and render: `busyType` and `boardParams` in
`KanbanView.vue`, the type filter in `resolvedCardFields`, and the `busy` branch
in `CardFieldList.vue`.
  - `RlBusyIndicator` and `useMinuteClock` (rela-components): one shared
minute timer for every indicator on the page.
  - `LeaseBadge` in the table's `#meta` slot: per-row state beside the
title (considered, see Alternatives).
  - Server side, unchanged: `serveRowBusy`, `recordBusy`, `selectsBusy`,
the safety replay, `liveListKeys`.
- Reference implementations: GitHub's pull request list shows a running
check as a small dot in its own column. Linear shows an agent chip in a list
cell. Both leave the details to the detail page.
- rela prior art: TKT-21G6JS (busy field, `?busy=`), TKT-W5ZSYJ (live
wake), the face column (TKT-WRLDAPI), TKT-4YPG4Q (lease badge in lists), and the
design-review findings `dr-21g6js.tsv` (validation in all three paths, gate
before fold) and `dr-w5zsyj.tsv` (tick barriers in negative live tests).

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

1. **Config shape: a column entry.**

   ```yaml
   lists:
     tickets:
       entity_type: ticket
       columns:
         - property: title
           link: detail
         - property: status
         - busy: run
           label: Agent
   ```

A list's display is declared in `columns:`, one entry per column. The face
column already sets the precedent for a column that shows row state rather than
a property. The kanban precedent is the same shape: `busy:` is an entry in
`card.fields`, not a key on the board. A column entry lets the author place the
column, label it and drop it, with the same keys as every other column. A
list-level key would need its own rules for position and header. See
Alternatives.

`config.go`: `ListColumn` gains `Busy string yaml:"busy" json:"busy,omitempty"`,
with a godoc that names TKT-TU3XH4 and points at `KanbanCardField.Busy` for what
"busy" means.

2. **Shape checks (`validate.go`).** In `validateLists`, before the face
branch, a busy column goes through a new `checkListBusyColumn(listID, i, c, seen
*bool) []string`:
   - refuse `Property`, `Relation`, `Direction`, `Face`, `Link` or
`Sortable` beside `Busy`;
   - refuse a second busy column;
   - refuse `i == 0`. The SPA renders column 0 as the row title through
the `name` slot, so a busy column there would leave the row without a title or
link.

Then `continue`, as the face branch does. `rollupMisuse` needs no change,
because it reads only `Property`.

A new `refuseListOnlyColumn(prefix string, c ListColumn) []string` returns "busy
is only supported on list columns" when `c.Busy` is set. It mirrors
`refuseKanbanOnlyFields`. It is called from two places only, both outside any
source or metamodel guard, so every column is checked once:
- *Dashboards.* In the per-card loop of `validateDashboardCards`
(`validate.go:2607`), for each of `card.Columns`. That loop visits every card of
the default and every named dashboard, whatever its source or display, and needs
no metamodel. `validateTableCard` and `validateDashboardCardSource` are not
touched: they skip query cards, cards without a metamodel and cards whose
display is not `table`.
- *View sections.* A new walker `refuseSectionBusyColumns(viewID, i,
s)` over `Columns`, `ParentColumns`, `ChildColumns` and every list in
`ColumnsByType` (sorted type order). It is called beside `validateSectionRender`
(`validate.go:1874`), before the `sourceType` and `GetEntityDef` guards, so a
section whose source does not resolve is still checked. `validateLevelColumns`
and `validateByTypeFallback` are not touched, since each would report the same
column again.

3. **Realtime checks (`channels.go`).** `busyFields` also walks
`cfg.Lists` in sorted id order, after the kanbans. Each busy column adds
`busyField{at: fmt.Sprintf("list %q: column[%d]", id, i), channelType: c.Busy,
entityType: list.EntityType, noun: "column"}`. `busyField` gains `noun`, set to
`"field"` for kanban card fields. Nothing else changes:
   - `busyFieldErrors` gives the same errors and the initial-working
warning;
   - the realtime-off warning reads "so this busy " + `noun` + " shows
nothing". A list column says "column", in line with its `at` prefix. The kanban
text stays "field", so the existing assertions in `realtime_wiring_test.go` and
`validate_channels_test.go` hold;
   - `HasRealtimePresentation` turns true for a list busy column. So
`projectsetup.CheckChannels` and `HasRealtimePresentation`, `rela validate`,
Configure's `Validate`, `SetRealtime` at boot and `checkChannelsPresentation` on
reload check it with no edit of their own.

Godoc touch-ups that name list busy columns: `ValidateChannels`,
`HasRealtimePresentation` (both packages), `CheckChannels`, the
`configureHost.Validate` comment and `reportRealtimeValidation`.

4. **Configure allowlist (`internal/configedit/allowlist.go`).**
   - Editable: `lists.*.columns.[].busy`. It is display only, like
`kanbans.*.card.**`, and the save runs `CheckChannels`. It restricts nothing, so
it is not `protective`.
   - Locked, next to the face keys: `dashboard.cards.[].columns.[].busy`
and `dashboards.*.cards.[].columns.[].busy`. The load refuses them anyway;
locking keeps the test's classification honest.
   - `views.*.sections.**` is already editable by wildcard; the load check
refuses `busy` there (AC 2).

5. **Export (`internal/dataentry/export_list.go`).** `listColumnsOf`
drops busy columns. An export is a document, and the busy state is a live,
per-principal channel state. Serving it would add the channel read to the export
path, which no one asked for. Column 0 cannot be busy (step 2), so the export
always keeps at least the title column.

6. **Server docs (`internal/dataentry/busy_rows.go`).** Reword the
`busyParam` godoc: "Off by default: a kanban card with a `busy:` field or a list
with a busy column asks for it." No code change.

7. **SPA types (`frontend/src/types`).**
   - `config.ts`: `ListColumn.busy?: string`, with a doc comment.
   - `entity.ts`: reword the `ListParams.busy` comment to name list busy
columns.

8. **SPA columns (`frontend/src/components/lists/tableColumns.ts`).**
   - `columnKey`: `busy` when `column.busy` is set, checked right after
`face`. One busy column per list, so the key is fixed.
   - `columnHeader`: `column.label || ... || (column.busy ?
t('list.column.busy') : '')`.
   - `sortable` is already false without a `property`.
   - `toTableColumns` sets `width: 160` on a busy column. That fits the
dots, a short state label, the time and "+1" ("Planning · 3h +1"). A longer
label truncates with an ellipsis, as the component already does. Without a width
the column takes the table default, which either truncates the label early or
wastes space.
   - i18n: `list.column.busy` is "Busy" in `en.json` and "Bezig" in
`nl.json`, next to `list.column.face`.

9. **SPA list (`frontend/src/components/lists/EntityList.vue`).**
   - `busyType = computed(() => listConfig.value?.columns?.find((c) =>
c.busy)?.busy)`.
   - `queryParams` sets `params.busy = busyType.value` when it is set. The
list query, the grouped loader (`listAllEntities`) and the live path all read
`queryParams`, so all three carry it. `listBaseParams` is not changed, so the
sidebar flyout, which shows no busy column, does not pay for the busy read.
   - `busyEntries(entity)`: the `_busy` entries whose `type` is the busy
type, mapped to `BusyIndicatorEntry` as `resolvedCardFields` does (`typeLabel:
b.type_label || b.type`, `label: b.label || b.state`, `since`). Extract that
mapping from `KanbanView.vue` into a small shared helper, `toBusyEntries(busy,
type)` in `frontend/src/utils/busy.ts`, and use it in both views.
   - The `#cell` slot gets a busy branch before the generic one:
`<RlBusyIndicator v-if="listColumnOf(column)?.busy" :entries="busyEntries(item)"
/>`. `RlBusyIndicator` renders nothing for an empty list. `isCellInaccessible`
and `resolveCell` are skipped for a busy column, since it names no property.
   - In the `tableColumns` computed: `isEmpty` returns
`busyEntries(entity).length === 0` for a busy column; `primary` is true for a
busy column, beside the face column. The comment there says why: busy is the
state a reader scans the list for.
   - `getFormattedCellValue` returns `''` for a busy column, so no code
path prints the channel type name as a value.
   - *Grouped lists.* Today `visibleListColumns` drops the grouped-property
column first, and `toTableColumns` then slices off the first remaining column as
the title, while `titleColumn` comes from the unfiltered list. When column 0
shows the grouped property, column 1 is sliced away. With a busy column at 1,
the list would pay for the busy read and show nothing. Fix: take the title
column off first, then filter the grouped-property column from the rest. This
also fixes the same drop for any other column (AC 14).

10. **Display decisions.**
    - *Position*: where the author puts the entry in `columns:`. Never
column 0.
    - *Header*: the column's `label`, else the i18n "Busy" or "Bezig".
The face column does the same. The raw channel type name (such as `run`) reads
like an identifier in a header that is always visible. Each cell's accessible
name already says the type label.
    - *Sorting*: none, and `sortable: true` is refused at load. A sort by
busy state would read the channel store for the whole result set before paging.
That breaks the one-batch-per-page rule (`collection-reads.md`). The order would
also be per principal and derived from channel content.
    - *Narrow and mobile*: `EntityList` always uses `compact="compress"`,
and `RlTable` measures its own width. A compressed row keeps only `primary`
columns, unlabelled. The busy column is `primary`, so on a phone or beside a
detail panel the row shows its title, its enum badges, its face and the
indicator. The indicator's label truncates with an ellipsis; the time and "+N"
never do (component behaviour from TKT-21G6JS). In the compressed layout an
empty primary cell keeps its slot on purpose (`RlTableRow.vue:611-623`), so rows
whose cells come and go do not jitter. The busy column follows that rule: an
idle row keeps an empty busy slot. That is the chosen behaviour, and the
`BusyColumn` story shows it. `isEmpty` only collapses the cell in the stacked
layout, which `EntityList` does not use; it is still set so the column behaves
right wherever `RlTable` stacks.
    - *Accessibility*: the indicator carries its own accessible name and
is not a live region. A table of many rows therefore announces nothing on
refresh, as on the board.

11. **Live refresh.** No server change, and no SPA change beyond sending
the parameter. The TKT-W5ZSYJ wake covers any live list path with `busy`:
    - `liveListKeys` accepts `busy`, `list_id`, `page`, `per_page`,
`sort`, `query_scope`, `scope_*`, `world` and the filters, so a list view's path
parses.
    - `serveRowBusy` records the wake from the served rows of that
request. A paged list subscribes its own page, so its wake is exactly the rows
on screen. AC 8 pins that for page 2, which a board never asks for.
    - The safety replay takes every busy subscription with a wake, on its
principal's slice, so a move from another process shows within one safety
period, as on the board.
    - A grouped list subscribes `page=1&per_page=100`, so only its first
100 rows are watched. Rows past them update on the next entity change or reload.
This is the board's limit, and the docs say so.
    - A list with a search term has no live path (`liveListPath` returns
`null` for `q`, because the server refuses `q` on a live path). Its indicators
keep counting minutes on the client clock but change state only on the next
refetch. The list request still carries `busy` (AC 6), and the docs name this
limit next to the grouped one.

12. **Library (rela-components).** No component change: `RlBusyIndicator`
takes the same entries, and `RlTable` renders any content in its `#cell` slot.
Add one story, `BusyColumn` in `RlTable.stories.ts`, that puts `RlBusyIndicator`
in a cell. It shows a busy row, an empty row and a "+1" row, in the normal and
the `compress` layout. It is the visual check for width and truncation in a
table, which the existing `RlBusyIndicator` stories do not cover.

13. **Configure SPA.** No editor control to add a busy column (out of
scope). The page must still handle one that exists.
    - `frontend/src/configure/models.ts`: `ColumnModel` gains `busy:
string` and `face: boolean`, read in `columnModel`. `columnTitle` returns
`c.label || c.property || c.relation || c.busy || 'column'`. A new
`columnSource(c)` returns `c.property || c.relation`, else `busy: <type>` for a
busy column, else `face` for a face column. A new `canSortColumn(c)` is false
for a busy or face column. Two guards, `columnMoveAllowed(rows, from, to)` and
`columnRemoveAllowed(rows, index)`, return false when the result would put a
busy column at index 0. Edits are path based, so reordering or relabelling a
busy column keeps its `busy` key.
    - `frontend/src/views/configure/ListView.vue`: the header placeholder
and the source text use `columnSource`. The Sortable switch renders only when
`canSortColumn` is true, so the page cannot write `sortable: true` on a busy
column, which the load refuses. The preview passes `sortable: canSortColumn(c)
&& c.sortable`. The `@move` handler calls `draft.moveAt` only when
`columnMoveAllowed` is true; otherwise the list keeps its order. The Remove
button of column 0 is disabled when `columnRemoveAllowed` is false. When a busy
column is in the list, the Columns section description adds "A busy column
cannot be the first column." `RlSortableList` has only a list-wide `disabled`,
so the guard sits in the page's move handler rather than in the component.
    - `frontend/src/configure/changes.ts`: the `ColumnModel` literal in
`listInner` reads `busy` and sets `face`, so the change review titles a busy
column by its type and the file compiles with the new fields.

**Alternatives considered:**

- *A list-level `busy: <type>` key.* It would mirror the board's one type
per request. Rejected: a list declares what it shows in `columns:`. A list-level
key would need its own rules for where the indicator goes and what the header
says, and a later "hide this column" would need a second key. The kanban field
is itself an entry in a field list, so a column entry is the closer mirror.
- *The indicator in the `#meta` slot beside the title, like
`LeaseBadge`.* No header, always visible, and it survives every layout. Rejected
as the default: the author cannot place or label it, and the title cell already
holds the world badge and the lease badge. A column that is `primary` gets the
same visibility in the compressed layout.
- *Derive `busy` on the server from `list_id`.* The request would not need
the parameter, and there is a precedent: `listRollups` derives per-row extras
from `list_id` and the list's columns in `serveListRowExtras`. Rejected: the
board already sends `busy` explicitly, and one mechanism keeps the board and the
list identical. The live path is the request path, so the parameter also makes
the subscription say what it asks for. The parameter is checked against the
snapshot's channel types either way.
- *Add `busy` to `listBaseParams`.* Rejected: the sidebar flyout uses it
and shows no busy column, so it would pay for a channel read per page it never
renders.
- *Header from the channel type name, as the kanban field's label
defaults.* Rejected: a card field hides its label by default, so the type name
is rarely seen there. A column header is always seen.
- *Show busy state in the list export.* Rejected: see step 5.
- *Sorting by busy.* Rejected: see step 10.

**Dependencies:** `internal/dataentryconfig`, `internal/configedit`,
`internal/dataentry` (export and a godoc), `internal/projectsetup`,
`internal/cli`, `cmd/rela-server` (godoc and tests). Frontend: rela-components
(story only), no new packages. It depends on TKT-21G6JS and TKT-W5ZSYJ, both on
`develop`.

**Files to modify:**

- `internal/dataentryconfig/config.go`, `validate.go` (shape checks, the
dashboard card loop, the section walker), `channels.go`.
- `internal/configedit/allowlist.go`.
- `internal/dataentry/export_list.go`, `busy_rows.go` (godoc).
- `internal/projectsetup/validate.go`, `internal/cli/validate.go`,
`cmd/rela-server/configure.go` (godoc only).
- `frontend/src/types/config.ts`, `frontend/src/types/entity.ts`.
- `frontend/src/utils/busy.ts` (new).
- `frontend/src/components/lists/tableColumns.ts`,
`frontend/src/components/lists/EntityList.vue`,
`frontend/src/views/KanbanView.vue` (use the shared helper).
- `frontend/src/i18n/locales/en.json`, `nl.json`.
- `frontend/src/configure/models.ts`, `frontend/src/configure/changes.ts`,
`frontend/src/views/configure/ListView.vue`.
- `frontend/packages/rela-components/src/components/table/RlTable.stories.ts`.
- Tests: `internal/dataentryconfig/validate_test.go`, `channels_test.go`
(including the docs example pin); `internal/configedit/allowlist_test.go`;
`internal/projectsetup/validate_test.go`;
`internal/cli/validate_channels_test.go`; `cmd/rela-server/configure_test.go`;
`internal/dataentry/realtime_wiring_test.go`, `live_busy_test.go`,
`busy_rows_test.go`, `export_list_busy_test.go` (new);
`frontend/src/components/lists/EntityList.busy.test.ts` (new),
`tableColumns.test.ts`, `frontend/src/utils/busy.test.ts` (new),
`frontend/src/configure/__tests__/models.test.ts`, `changes.test.ts`,
`frontend/src/views/configure/ListView.test.ts` (new).
- Docs: `docs-project/entities/guides/GUIDE-data-entry.md`,
`GUIDE-realtime.md`, then `just docs`; `docs/data-entry/api-reference.md`.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

- *`busy:` on a list column (operator).* Checked at boot, on reload, by
`rela validate` and by Configure against `realtime.yaml` (AC 3, AC 4). The
channel type must be declared, have a working session state and attach to the
list's entity type. A failed reload keeps the last good config. A failed
Configure save is refused with the problem list.
- *`?busy=` (client).* Unchanged from TKT-21G6JS: the first value, looked
up in the snapshot's channel types. An unknown name serves no `_busy`. The list
view only makes the SPA send a name the config already holds.
- *Configure.* `lists.*.columns.[].busy` is editable by key. The busy key
restricts nothing and runs no code, so it is not `protective`. The dashboard
keys stay locked.

**Security-Sensitive Operations:**

- *Row gate.* `_busy` rides on rows the list already gated and redacted.
A list view adds no read path: it calls the same endpoint as the board, and
`list_id`, scopes and filters only narrow the rows (AC 9).
- *Channel gate.* Unchanged. `BusyReader` gates each channel through the
anchor reader and the type's `read:` role. A denied channel reads as not busy
(`[]`), the same as an idle one.
- *Gate before fold.* The busy column folds nothing. It shows each row's
own gated entries. No count, sort, group or truncation flag is derived from busy
state, and sorting by it is refused at load. So no hidden channel can move a
visible value or position.
- *No new oracle.* The only new request-time difference is that a list
view sends `busy`. The answer per principal and the live frames are the ones
TKT-21G6JS and TKT-W5ZSYJ already review. The accepted one-bit timing residual
in `live_busy.go` applies to a list as to a board; a list adds no new kind of
subscription.
- *Config is not a secret.* The busy type name appears in the served
list config, as the kanban field does. That is expected (CLAUDE.md).
- *Rendering.* Labels render through Vue interpolation in
`RlBusyIndicator`, never `v-html`.
- *Export.* The busy column is dropped, so the export path gains no
channel read and no per-principal channel content.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**

| AC | Test |
| --- | --- |
| 1 | `TestValidateConfig_ListBusyColumn` |
| 2 | `TestValidateConfig_BusyColumnOnlyOnLists` |
| 3 | `TestValidateChannels_BusyListColumn` |
| 4 | `TestValidate_BusyListColumnWithoutChannels`, `TestCheckChannels` case, `TestConfigure_BusyFieldAgainstRealtime` list case, `TestSetRealtime_BusyListColumn`, `TestReloadConfig_ChecksBusyField` list case |
| 5 | `TestCheckEdit` cases, `TestAllowlist_ClassifiesEveryKey` |
| 6 | `EntityList.busy.test.ts` request, live and search cases |
| 7 | `EntityList.busy.test.ts` render cases, `tableColumns.test.ts` cases (including width), `busy.test.ts` for `toBusyEntries` |
| 8 | `TestLive_BusyListPagedWake` |
| 9 | `TestListBusy_ListIDScoped` |
| 10 | `TestExportList_SkipsBusyColumn` |
| 11 | `models.test.ts` source, title and guard cases; `changes.test.ts` review title; `ListView.test.ts` switch, source and move cases |
| 12 | `RlTable.stories.ts` `BusyColumn` in the Storybook smoke run |
| 13 | Reviewed; `just docs`; `TestDocsExample_BusyListColumn` |
| 14 | `EntityList.busy.test.ts` "grouped by the first column keeps the busy column" |

Integration: `TestLive_BusyListPagedWake` runs the HTTP list handler, the live
hub and the in-memory channel store with a list-view path.
`TestConfigure_BusyFieldAgainstRealtime` runs the Configure validate path end to
end. `TestReloadConfig_ChecksBusyField` runs the `data-entry.yaml` reload with a
list busy column. `KanbanView.cardBusy.test.ts` must keep passing after the
shared helper is extracted, and the existing grouped list tests must keep
passing after the column filter fix.

**Edge cases:**

- A busy column with `label` only: header shows the label.
- A row with `_busy: []`: empty cell, `isEmpty` true.
- A response without `_busy` (realtime off, degraded config, a type
removed after a reload): every cell empty, the list still renders.
- Entries of a type other than the busy type: ignored.
- Two or more entries: first entry plus "+N", every entry in the name.
- A grouped list: each page carries `busy`; only the first 100 rows are
watched live.
- A list on a faced type under a world: a family channel shows on each
face row, a face channel only on its own face's row (server behaviour,
unchanged).
- A list with a face column and a busy column: distinct keys (`face` and
`busy`) and both `primary`.
- A list grouped by the property of its first column, busy column second:
the busy column still renders (AC 14).
- A list with a search term: the request carries `busy`, no live path,
state changes on refetch only.
- Compressed layout, idle row: the busy slot stays empty and in place.
- Configure: dragging a busy column to the top, or removing column 0
above it, is not applied.
- A list whose config is replaced by a reload that removes the busy
column: the next request has no `busy`, and the live path changes, so the old
subscription is dropped (existing `useLiveSubscription` behaviour).
- `show_label` on a list column: not a `ListColumn` key, so it does
nothing, as for any other column. The column header is the label.

**Negative Tests:**

- `busy` with each of `property`, `relation`, `direction`, `face`,
`link`, `sortable: true`: refused (AC 1).
- Two busy columns, and a busy column at index 0: refused (AC 1).
- `busy` on dashboard and view columns, including a query card and a
section whose source does not resolve: refused once (AC 2).
- A reload with `busy: nope`: refused, last good config kept (AC 4).
- Configure: no Sortable switch on a busy row, no move to the top (AC 11).
- Undeclared type, no session, no working state, wrong entity type:
refused on every check path (AC 3, AC 4).
- Dashboard busy key edited in Configure: refused as locked (AC 5).
- A move on a row outside the subscribed page: no replay (AC 8).
- A row the list scope drops: not in the busy read (AC 9).

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

- *`ListColumn` is shared by four surfaces.* A new field is accepted in
YAML on all of them. Mitigation: the refusal on dashboards and views (AC 2), and
the locked Configure keys (AC 5).
- *Cost on large grouped lists.* A grouped list loads up to `max_rows`
(default 500, ceiling 2000) in pages of 100, with one busy read per page. Worst
case: 20 `ListWaiting` calls per load, and 20 more on each live frame, since a
frame refetches the whole set. On the file backend each call reads one directory
per anchor, so a 2000-row list costs about 2000 directory reads per refresh. The
safety replay also re-serves each open grouped tab's 100-row live path once per
safety period, with one busy read. Accepted for this ticket: the read is opt-in
per list, the cost per page is the board's, `channels_budget_test.go` pins one
call per page, and a 2000-row grouped list already costs 20 page reads without
busy. Follow-up idea if it shows up in practice: send `busy` only on the first
100 rows of a grouped list, which are the rows the live path watches anyway.
- *Live replays.* Every open list with a busy column joins the busy wake
and the safety replay. A paged list's wake is smaller than a board's (a page,
not 100 rows), so this adds subscriptions but not larger ones. The
sustained-move cost in the TKT-W5ZSYJ risks applies per open list.
- *Drift between board and list rendering.* Mitigation: one shared
`toBusyEntries` helper, tested once.
- *Configure writes a column the load refuses.* Mitigation: no Sortable
switch on a busy column, and the move and remove guards (AC 11). Other invalid
raw edits are still caught at save by the load checks.
- *The grouped-column fix changes existing grouped lists.* It only stops a
column from being sliced away by mistake. Mitigation: the existing grouped list
tests plus AC 14.
- *Security risk:* low. No new read path, no fold, no new subscription
kind.

**Effort:** m. About 70 lines of Go config and validation, a handful of
allowlist and export lines, about 50 lines in the list SPA, about 40 lines in
the Configure page and models, the grouped-column fix, one story, about fifteen
test additions (one new view test) and two guide sections. The design review
moved it from s to m.

## Documentation Planning

For enhancements: identify what documentation needs updating.

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**

- [x] `docs-project/entities/guides/GUIDE-data-entry.md` (generates
`docs/data-entry.md`), "Column Options": add `busy` to the table ("Shows a busy
indicator while a channel of this type on the row is in a working state; lists
only"), change "Set exactly one of `property`, `relation` or `face`" to include
`busy`, and add a "Busy column example" with the rules table: at most one per
list, not the first column, stands alone, not sortable, lists only, and the
channel type checks. Note the default header "Busy" and that the export leaves
the column out. Link "Busy cards" and the column example to each other.
- [x] `docs-project/entities/guides/GUIDE-realtime.md` (generates
`docs/realtime.md`), "Busy indicator": say that a board card field or a list
column turns it on, with a short list example. Drop "A list view has no busy
column yet (TKT-TU3XH4)". In "When it updates", say that a paged list watches
the page on screen, a grouped list its first 100 rows, and a list with a search
term does not update live: its indicators change state on the next refetch.
- [x] `docs/data-entry/api-reference.md` (hand-written): "A kanban card
with a `busy:` field sends it" becomes "A kanban card with a `busy:` field, or a
list with a busy column, sends it."
- [x] ~~docs/metamodel.md~~ (N/A: no schema change)
- [x] ~~docs/cli-reference.md~~ (N/A: no command change)
- [x] ~~CLAUDE.md~~ (N/A: no new pattern)
- [x] ~~README.md~~ (N/A: no project-level change)

## Decisions

- **Header default.** The i18n "Busy" or "Bezig", like the face column.
- **Configure editor.** The allowlist admits the key. The column editor
gets no busy kind; the page only handles a busy column that exists.
- **Column 0.** A busy column as the first column is refused at load.

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** 13 findings (2 significant, 6 minor, 5 nit). All are
addressed in the plan; none was declined. Changes:

- *Configure list page (significant).* `ListView.vue` and `changes.ts`
join the file list. A busy or face column gets no Sortable switch, its source
text reads `busy: <type>`, the preview marks it not sortable, a move or removal
that would make a busy column first is not applied, and the change review titles
it by type. AC 11 and its tests grow, with a new `ListView.test.ts`.
- *Refusal call sites (significant).* Dashboards: one call per card in
the `validateDashboardCards` loop. Sections: one walker beside
`validateSectionRender`, outside the source guards. AC 2 gains a query card, a
non-table list card, an unresolvable source and a single-report check.
- *Grouped list drops the busy column.* The title column is taken off
before the grouped-property filter. New AC 14.
- *Compressed layout.* The text is corrected: an empty busy cell keeps
its slot in `compress`, by choice, and the story shows it.
- *Reload.* A list case in `TestReloadConfig_ChecksBusyField` (AC 4).
- *Searched list.* Documented as a live limit; a test pins that the
request still carries `busy` (AC 6).
- *Grouped cost.* Worst case stated (20 reads per load, about 2000
directory reads per refresh on the file backend) and accepted, with a follow-up
idea.
- *Docs example.* `TestDocsExample_BusyListColumn` pins the block, like
the channels example (AC 13).
- *Nits.* `entity_views` dropped from step 4. The server-derived
alternative now cites `listRollups` and gives the real reason. The busy column
gets a fixed width of 160 px. The realtime-off warning says "column" for a list
column through a `noun` on `busyField`. The Configure move guard covers moving a
busy column to the top.
- *Effort* moves from s to m.
