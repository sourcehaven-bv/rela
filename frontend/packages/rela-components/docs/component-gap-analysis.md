# Component gap analysis

What the `rela-style-update` frontend (`frontend/src`, 94 single-file
components) has that this library does not. Written to decide which components
to adopt, not as a build plan.

Compared against the 74 components in `src/components` on 2026-09-22. Sections
4, 5, 7 and 8 record what has been adopted since, so the library now holds 89.

## How to read this

The gaps fall into two kinds, and the distinction matters more than the
individual rows.

Some rows are missing **primitives**: this library has nothing that plays that
role. Others are missing **assemblies**: the primitives are all here, but
nothing composes them. A command palette is `RlModal` plus `RlSearchBox` plus
keyboard handling; a filter bar is `RlFilterChip` plus a menu. Roughly half of
the table below is the second kind.

Deciding whether this library carries assemblies, or stops at primitives and
leaves assembly to the app, resolves about half of these rows at once.

## 1. Generic UI primitives

The clearest candidates. Each is app-agnostic and has no schema or domain
coupling. Section 8 records the four adopted from here.

| Component             | Source                                  | What it does                                                                                                | Nearest existing                            |
| --------------------- | --------------------------------------- | ----------------------------------------------------------------------------------------------------------- | ------------------------------------------- |
| `RlCommandPalette`    | `ui/CommandPaletteModal.vue` (464 l)    | Cmd/Ctrl+K quick jump: filter a list, arrow-key highlight, Enter to act                                     | `RlModal` + `RlSearchBox`, unassembled      |
| `RlKeyboardShortcuts` | `ui/KeyboardShortcutsModal.vue` (201 l) | Shortcut reference sheet, grouped rows of keys and descriptions                                             | `RlKbd` only                                |
| `RlStatusBar`         | `common/StatusBar.vue` (433 l)          | Bottom app chrome: the quietest prominence tier, a chip that expands on click                               | `RlActivityBar`, a different role           |
| `RlSidePanel`         | `forms/SidePanel.vue` (444 l)           | Docked side panel for forms and detail                                                                      | `RlDrawer`, which is an overlay, not docked |

## 2. Lists and filtering

The largest gap at the assembly level. The primitives exist; the things that
compose them do not.

| Component              | Source                                  | What it does                                                                                               | Nearest existing                         |
| ---------------------- | --------------------------------------- | ---------------------------------------------------------------------------------------------------------- | ---------------------------------------- |
| `RlFilterBar`          | `lists/FilterBar.vue` (550 l)           | Active-filter bar: chips, add, clear, sort controls                                                        | `RlFilterChip` only                      |
| `RlFilterMenu`         | `lists/AdHocFilterMenu.vue` (487 l)     | Two-step property then value picker; enum dropdown for properties that declare values, free text otherwise | `RlMenu` + `RlSelect`, unassembled       |
| `RlEntityList`         | `lists/EntityList.vue`                  | Configurable data list, the layer above a table                                                            | `RlTable`, presentational only           |
| `RlIssuesTable`        | `common/IssuesTable.vue` (467 l)        | Table whose rows have split click targets: the title cell navigates, the message cell reveals detail       | `RlTable`, single row-click model        |
| `RlTagSelect`          | `ui/TagSelect.vue` (126 l)              | Multi-select with per-option disabled verdicts and a read-only mode                                        | `RlMultiSelect`, no per-option verdicts  |
| `RlEntityTargetSelect` | `common/EntityTargetSelect.vue` (334 l) | Single-target picker over a pre-resolved candidate list                                                    | `RlSelect`, no search or candidate model |

## 3. Field and property display

| Component           | Source                                | What it does                                                                                          | Nearest existing                  |
| ------------------- | ------------------------------------- | ----------------------------------------------------------------------------------------------------- | --------------------------------- |
| `RlPropertyDisplay` | `common/PropertyDisplay.vue` (145 l)  | One read-only property row on a 12-column grid; width-aware, and long-form values take the full width | `RlDetailField`, simpler, no grid |
| `RlCardFieldList`   | `common/CardFieldList.vue` (106 l)    | Label and value lines shared by kanban cards and calendar chips                                       | inline markup in `RlTaskCard`     |
| `RlLockedField`     | `common/InaccessibleField.vue` (35 l) | The lock affordance when a value is unreadable, through permissions or encryption                     | none                              |
| `RlFieldRenderer`   | `forms/FieldRenderer.vue`             | Routes a schema field to the right widget                                                             | none                              |
| `RlDynamicForm`     | `forms/DynamicForm.vue`               | Schema-driven form assembly                                                                           | none                              |
| `RlRruleBuilder`    | `forms/RruleBuilder.vue` (443 l)      | Recurrence-rule builder                                                                               | none                              |
| `RlStatusControl`   | `forms/StatusControl.vue` (197 l)     | Status picker offering only the reachable transitions, not every enum value                           | `RlStatusPill`, display only      |

## 4. Comments and collaboration

Adopted. All four are in `src/components/comment`, alongside
`RlCommentThread` and `RlCommentComposerBox`, which the others share.

They are presentational, as the rest of the library is: comments come in as
props and every change goes out as an event, so the app keeps the fetching, the
permissions and the anchor resolution. Three consequences of that are worth
knowing before using them.

The library does not know what an anchor points at. A `CommentAnchor` carries a
kind, a reference and a label, and the app decides what those mean. That is what
let these be adopted without the schema contract section 3 is still waiting on.

`RlTextSelectionComment` reports the selected text and the rendered text either
side of it, and nothing more. Mapping a quote back to a position in the source
belongs to whoever owns the source. It also takes a `blockedReason`, so a
selection that cannot be anchored says so before anything is typed rather than
failing on save.

`RlBlockCommentOverlay` is given the commentable elements rather than finding
them: which parts of a rendered document can be commented on is a property of
that document, not of the overlay.

## 5. Calendar

Adopted. All three are in `src/components/calendar`, alongside
`calendarGrid.ts`, the day arithmetic they share.

The view family came over because it is a primitive, not an assembly: a month
grid has the same shape in every app that draws one, which is the rule section
8 states. What the app keeps is everything that decides WHAT goes in a cell.

### The library owns the grid; the app owns the mapping onto it

`calendarGrid.ts` is pure arithmetic over a `CalendarDay`, a timezone-free
`{ year, month, day }`. It builds month and week grids, steps the anchor a
period at a time, and derives weekday headers from the platform's locale.

There is deliberately no conversion from a stored value to a day. Deciding
which cell `2026-03-01T00:30:00Z` falls in needs a display timezone, and the
app owns both that and the storage format. Putting it here would make the
library either guess the browser's zone, which disagrees with a configured
display zone and places an event in a cell whose printed time contradicts its
position, or take a timezone dependency for a decision it is not entitled to
make. The source's `eventDay`, `eventMinutes`, `applyDayDelta` and
`windowBounds` all stay in the app for that reason, and with them the only
reason `@date-fns/tz` was needed.

So `RlCalendarGrid` is given days and events, never instants.

### An event arrives positioned and formatted

`CalendarEvent` carries a summary, an inclusive `startDay` and `endDay`, an
already-formatted `timeLabel`, a colour, and detail lines as plain label and
value strings. The source's chip resolved those lines through a widget
registry and a schema store, which is the schema contract section 3 is still
waiting on; taking pre-resolved strings is what let this be adopted without it.

`startDay` and `endDay` are the event's real extent, not clipped to the
visible period. That is what lets a chip read as a continuation on the first
visible day rather than appearing to start there.

`draggable` on an event is an affordance, not an authorisation: the library
draws the grab handle, and the app decides who gets one and re-authorises the
write.

### What changed in adoption

A multi-day event is still drawn as one chip per day rather than a bar across
the row, and the chip keeps the source's `→ ← ↔` continuation markers, with
the arrow decorative and a sentence given to screen readers. Three identical
chips in a row would otherwise read as three separate events.

The grid buckets events by day itself. The source had the parent supply both
the capped list and the hidden count per cell; here the grid takes `maxPerDay`
and derives the rest, placing each event by its offset from the first visible
day rather than having every cell filter the whole event list.

Chip and swatch colours map onto the existing tag palette rather than a
private set, so a calendar restyles with the rest of the library.

`RlCalendarLegend` identifies sources by id rather than by array index, so
hidden ids can live in a URL and still mean the same thing after the source
list is reordered.

## 6. Domain-specific

Probably stays in the app. Listed so the decision is explicit rather than an
omission.

| Component                                                                                                          | Source                           | Note                                                                                                                    |
| ------------------------------------------------------------------------------------------------------------------ | -------------------------------- | ----------------------------------------------------------------------------------------------------------------------- |
| `FaceMenu`, `CopyMenu`, `WorldBadge`, `WorldBanner`                                                                | `entity/`, `common/`             | Tied to the faces and worlds model. The generic shapes underneath are `RlMenu` and `RlBanner`, both of which exist here |
| `ExportMenu`, `DuplicateModal`, `CommandModal`, `InlineCreateFormModal`, `EntityPickerModal`, `EntityPreviewModal` | `entity/`, `forms/`, `calendar/` | Instances over `RlModal`. Adopt the patterns, not the components                                                        |
| `ScriptErrorDialog`, `ScriptErrorPanel`                                                                            | `common/`                        | Lua script error surfacing, deployment-specific                                                                         |
| `NextActionCard`, `NextActionOffers`                                                                               | `components/`                    | Suggestion prominence tiers. The tier system itself may be worth generalising                                           |
| `MentionMenu`, `entityRefNode`, `writeBackGuard`                                                                   | `forms/milkdown/`                | The app-coupled half of the editor. See section 7: the generic half was adopted as `RlMarkdownEditor`                   |

## 7. Markdown editor

Adopted, as `RlMarkdownEditor` in `src/components/editor`, alongside
`RlEditorToolbar` and `RlBlockIcon`.

The source `MilkdownEditor` mixes two things: a generic WYSIWYG markdown
editor, and rela's own concerns (entity references, `@` mentions resolved
through the schema store, a write-back guard tied to the on-disk corpus, a
Pinia store for error reporting). The split between them is what decided the
adoption boundary. The library took the first and left the second.

What came over is the markdown round trip and the toolbar state derived from
it: the commonmark and GFM presets, the command table, the active-formatting
probes, the dry-run availability check, the table operations, and the task-list
node view that draws a real checkbox.

What did not is anything that needs to know what a document MEANS. Notably
there is no write-back guard. Whether re-serialized markdown may be saved over
the original depends on what the original is worth, and a git-backed corpus and
a draft in local state want opposite answers, so the editor reports what it
holds and the app decides. `serializerContract.ts` exports `isSemanticallyEqual`
for apps making that call.

The app layers its own concerns back on through three hooks, which are meant to
be used together:

| Hook              | For                                                                       |
| ----------------- | ------------------------------------------------------------------------- |
| `plugins`         | App Milkdown plugins and nodes, joining the editor's own schema           |
| `#overlays`       | App UI positioned against the live view, such as a mention menu           |
| `#toolbar-extra`  | App toolbar buttons, given the same active and unavailable state          |

`@milkdown/kit` is a **peer** dependency for this reason. A second copy would
give the app a different ProseMirror `Schema` class, and its nodes would be
rejected by the document this editor builds.

This does not settle the assemblies question below. The editor is a primitive
that happens to be large: it composes no other `Rl*` component, and its size is
the markdown round trip rather than a policy about how the app fits together.

## 8. Small primitives

Adopted: `RlHelpButton`, `RlBackButton`, `RlNavIcon`, and the pending state,
which went into `RlButton` rather than arriving as its own component. These
were taken together because they are the only rows that neither open decision
below blocks.

`RlPendingButton` is deliberately absent. `RlButton` already had `pendingLabel`,
and `RlActivityBar` already named it as one of the three pending indicators, so
a second button differing only in its gating would have left two, with the
wrong one more discoverable. What the source had and `RlButton` did not was the
behaviour around the swap, which is now in `RlButton`:

- The swap waits behind `useDelayedPending`, so an action that finishes inside
  the delay shows nothing at all. This is the anti-flash rule `RlActivityBar`
  applies to navigation, with a longer delay because a label changing under the
  cursor is more invasive than a bar at the edge of the screen.
- Both labels render stacked in one grid cell, so the button cannot resize
  mid-action. The spinner's box is reserved for the same reason when there is
  no icon to replace.
- `aria-disabled` rather than native `disabled` while pending, because native
  disabled drops focus to `<body>` and strands a keyboard user mid-action.
  Activation is suppressed in the handlers instead, keyboard included.

`useDelayedPending` is exported, since an app wiring its own indicator wants
the same gate.

`RlBackButton` takes its label as a prop. Resolving what the previous place is
called needs the router and the app's titles, which is the same boundary
section 4 drew for comment anchors.

`RlNavIcon` carries no icon registry: this library has a typed `IconName` union
rather than config-authored strings, so the source's `NO_ICON` sentinel becomes
an explicit `reserve` prop. `RlSidebarNavItem` now uses it for the case where an
item has neither icon nor badge, which previously left no box and let the label
jump left of its siblings.

`RlHelpButton` opens `RlModal` rather than the source's hand-rolled overlay, so
the focus trap, Escape handling and overlay stack are the library's existing
ones. Help content is a slot: fetching it would make the library know where help
lives.

## Already covered

No action needed. These have direct counterparts here.

| In `rela-style-update` | Here                      |
| ---------------------- | ------------------------- |
| `ProjectSwitcher`      | `RlWorkspaceSwitcher`     |
| `Sidebar`              | `RlSidebar` and its parts |
| `PageLayout`           | `RlAppShell`              |
| `PageTitle`            | `RlPageHeader`            |
| `Badge`                | `RlTag`, `RlSidebarBadge` |
| `Toast`                | `RlToast`, `RlToastHost`  |
| `ActivityBar`          | `RlActivityBar`           |
| `Pagination`           | `RlPagination`            |
| `SearchBox`            | `RlSearchBox`             |
| `AutoSaveIndicator`    | `RlAutoSaveIndicator`     |
| `FieldShell`           | `RlFieldShell`            |
| `ConfirmModal`         | `RlConfirmDialog`         |
| `widgets/*`            | the `form/` components    |

## Two decisions this list depends on

**Does the library carry assemblies?** The command palette, filter bar and
issues table are all compositions of primitives that already exist here. If the
library stops at primitives, those rows become app code and the adoption list
shrinks by about half.

Section 4 answers this one way: the comments panel was adopted as an assembly.
It is not a precedent for the rest, because a comment thread has one shape
wherever it appears, while a filter bar's shape follows the list it filters.

Section 8's `RlHelpButton` is an assembly too, and it points at the rule those
two cases share: the library carries an assembly when the composition's shape
is fixed by the thing itself rather than by the surrounding app. A help button
is a button and a modal in every app that has one. Under that rule the command
palette and the shortcut sheet come in, and the filter bar and issues table
stay out. Stated here as the candidate rule, not as the decision.

**Does the library take on a schema contract?** `RlEntityList`, `RlDynamicForm`
and `RlFieldRenderer` are schema-driven. They cannot be adopted without the
library knowing about a schema, which it currently does not. This is the largest
single decision here, and it also governs `RlPropertyDisplay`, `RlTagSelect` and
`RlStatusControl`, all of which carry schema-derived concepts (per-option
verdicts, reachable transitions, property widths).
