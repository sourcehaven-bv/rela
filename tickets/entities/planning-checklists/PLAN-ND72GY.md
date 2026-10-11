---
id: PLAN-ND72GY
type: planning-checklist
title: 'Planning: Gantt: drag bars to reschedule, navigate with Now and arrows'
started: "2026-10-09"
completed: "2026-10-09"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

In scope (decisions from the user, 2026-10-09):

- Horizontal scroll with a day-linear scale: every day has one fixed width
per zoom (Week 40px, Month 12px, Quarter 4px). A short span still fills the
width. The axis row is sticky at the top and the tree column sticky on the left
inside a two-axis scroll container with a max height.
- Navigation: "Now" scrolls today into view; "‹"/"›" scroll by one zoom unit
(7 days, one month, three months). The existing Quarter/Month/Week buttons stay;
"Week" in the mockup is that zoom. The axis is widened to include today, capped
at one zoom unit's worth of padding beyond the plan; when today is further out,
Now is disabled with a hint.
- Move and resize. Dragging the entity's own planned window moves start and
end by the same number of days; dragging an edge changes only that date. One
PATCH per gesture.
- Who may drag (user decision): checked lazily. When the pointer rests on a
bar (150ms) or the bar takes focus, the SPA GETs that entity once and caches the
verdict per address. Handles appear when `_actions.update` is true and both
mapped date fields are writable (`_fields`) and readable. The commit re-reads
the entity and sends the PATCH with field preconditions (`_versions`), so the
server re-checks everything at write time.
- Keyboard (slider pattern): the start handle, end handle and move control
are focusable; arrows adjust a preview, Enter commits one PATCH, Escape cancels.

Out of scope: auto-scroll while dragging past the edge, moving children with a
parent, drawing new bars, undo.

**Acceptance Criteria:**

1. Dragging an editable bar two days right PATCHes start and end +2 days in
one request; the bar keeps its preview until the refetch shows the new dates.
2. Dragging the right edge changes only the end, the left edge only the start;
the end cannot pass the start (minimum one day).
3. No handles appear for a reader without update rights, for a node whose date
field is read-only or hidden, or for a node without its own dates; a rolled-up
span is never a drag target.
4. A click without movement still drills in; a click that ends a drag does not.
5. A span wider than the screen scrolls sideways with the tree column and axis
fixed; Now brings today into view; the arrows scroll one unit; scroll position
survives a refetch and a zoom change (anchored on the left-edge day).
6. A failed or conflicting PATCH (403, 412, 422) shows a message and the bar
returns to the server's dates; a 200 with warnings shows them.
7. Docs describe dragging, keys and navigation.

## Research

- [x] For larger features: run `/research` to create a structured research doc
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A: the user chose the approach; the patterns exist in the
codebase.

**Existing Solutions:**

- `CalendarView.vue` drag-to-reschedule (`onDrop`): `applyDayDelta` and
`daysBetween` in `utils/calendarGrid.ts`, one PATCH for start and end, the
optimistic helpers in `queries/optimisticList.ts`, and `canUpdate` from
`_actions`. Reused.
- `utils/ganttLayout.ts` `scaleFor` (equal-width periods); it needs an
inverse to turn a pointer x back into a day.
- Gantt libraries (frappe-gantt, dhtmlx) were not adopted: the server-folded,
ACL-gated tree and the breach rendering are rela-specific, and the change is a
gesture layer, not a new chart.
- Prior art: TKT-MW28U5 (gantt), the gate-before-fold rule in CLAUDE.md.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

Server (minimal): `GanttNode.Face` (`face`, omitempty) so the SPA addresses the
face the gantt shows (`id@face`). No ACL work on the gantt path; its cost is
unchanged.

Frontend:

- `ganttLayout.ts` gains a linear scroll scale: `pxPerDay(zoom)`,
`xForDay`, `dayForX`, `scrollUnitDays(zoom)`, ticks at stride 1 in scroll mode,
and px-based bar minimums (replacing the 0.6% and 60% rules there). Gridlines
become one background layer behind all rows.
- `GanttView.vue`: scroll container, sticky axis and tree column (opaque
backgrounds), fixed-width timeline, Now/‹/› buttons, scroll anchoring across
refetch and zoom, a sequence token on `fetchScope` (out-of-order responses
dropped; a failed refetch keeps the old data), refetch of `fetchedRoot`.
- The planned window is always rendered for a draggable node and is the move
target; edge handles of at least 8px hit area sit on its edges; `touch-action:
none` only there.
- New `composables/useGanttDrag.ts`: lazy verdict cache (Map by address,
cleared after any write or refetch), pointer gesture with 4px threshold, pointer
capture, cancel on Escape/pointercancel/lostpointercapture, click suppression
after a real drag, preview state, commit: GET entity → recheck verdict →
`applyDayDelta` per mapped property using the date kind of the schema property →
`updateEntity` with `preconditions` from `_versions` → refetch gantt and
invalidate `entityKeys.list(type)`. Refetches are deferred while a gesture is
active.
- Keyboard handlers on the focusable handles share the same preview and
commit path.

Alternatives rejected (design review RR on this plan): a per-node server check
(store queries per row up to max_nodes, and it misses field locks); sending only
`Planned` (datetime loses its time); HTML5 drag-and-drop (no resize, no live
preview); equal-width months (non-linear drag maths).

**Files to modify:**

- `internal/apiwire/v1/responses.go`, `internal/dataentry/gantt_handler.go`,
gantt handler test (face on a faced node)
- `frontend/src/api/gantts.ts`, `frontend/src/utils/ganttLayout.ts` (+ test),
`frontend/src/composables/useGanttDrag.ts` (new, + test),
`frontend/src/views/GanttView.vue`, `frontend/src/views/GanttView.test.ts`
- `e2e/tests/gantt-drag.spec.ts` (new), `e2e/pages/` gantt page object (new),
fixtures
- `GUIDE-data-entry.md` → `docs/data-entry.md`, `api-reference.md` (`face`)

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

- The write is an ordinary PATCH through `entitymanager`; ACL, field write
grants, validation and audit apply unchanged. The SPA only proposes values.
- Field preconditions (`_versions`) turn a concurrent edit into a 412 instead
of a lost update.

**Security-Sensitive Operations:**

- The hover verdict is a UI hint; the PATCH re-authorizes.
- `face` on a gantt node is the face the reader already read; no new data.
- The verdict cache is per page session and per principal (the SPA's
session); it is cleared after writes and refetches.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**

- Server: a faced gantt node carries `face`; a faceless one omits it.
- `ganttLayout`: `dayForX(xForDay(d)) == d`, tick stride 1, px minimums,
scroll unit per zoom.
- `useGanttDrag`: move, resize each edge, clamp, threshold click vs drag,
click suppression, Escape/pointercancel cancel, verdict cached (one GET for
repeated hovers), no handles when `update` false / field read-only / field
hidden, commit sends preconditions, 412/403 reverts with a message, 200 with
warnings shows them, datetime keeps its time.
- GanttView: timeline width per zoom, Now/arrows scroll offsets, scroll
anchor kept across refetch and zoom, stale fetch dropped, failed refetch keeps
data.
- e2e: hover then drag a bar two days and assert dates via the API; drag the
right edge; keyboard move + Enter; Now scrolls today into view.

**Edge Cases:**

- One-sided sources (only start or only end mapped): move shifts that value;
a bar renders only when today's layout draws one.
- Unquoted YAML dates arrive as datetime strings: the kind comes from the
schema property, and `applyDayDelta` parses date-kind values by day.
- Zero delta: no request. Drill or zoom during a drag: gesture cancelled.

**Negative Tests:**

- Verdict false ⇒ no handles, no tabindex, pointerdown never starts a drag.
- Verdict true at hover but write denied at commit ⇒ message, revert.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

- Drag vs click (drill) conflict: a 4px threshold, with a test for each.
- GanttView is already 1045 lines: the gesture logic goes into a composable.
- Hover GETs while sweeping the mouse: a 150ms dwell and a per-address cache
bound them to bars the user pauses on.
- A wide day-linear timeline at Week zoom over a multi-year plan is long:
gridlines are one layer and minimums are px, so cost is per row, not per tick.

Effort: l.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**

- [x] docs/data-entry.md (via GUIDE-data-entry.md): gantt dragging, keys,
navigation
- [x] docs/data-entry/api-reference.md: gantt node `edit`

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** cranky design review on 2026-10-09: 2 critical
(per-node ACL cost; update ≠ field writable) resolved by the user's lazy hover
check plus commit-time GET; 9 significant (normalized values, one-sided blind
writes, planned window target, drag/click, non-linear scale, % geometry,
sticky/scroll anchoring, keyboard per-press writes, refetch ordering) folded
into scope, approach and tests above; minors (preconditions, warnings toast,
today widening cap, new e2e files) folded in.
