---
id: PLAN-VSXB42
type: planning-checklist
title: 'Planning: Hierarchical Gantt view for data-entry (gantts: config, recursive roll-up, drill-down)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

IN — a `gantts:` view type in `data-entry.yaml`: config struct + validation +
normalization, a server-side tree/roll-up endpoint, and a `GanttView.vue`
rendering an outline tree against a time axis with drill-down navigation,
temporal zoom, peek-ahead, and planned-vs-rolled breach rendering.

OUT — dependency arrows, drag-to-reschedule, critical-path scheduling (all on
FEAT-AS2VRW). Also out, per design review: `multi_parent: duplicate` — roll-up
double-counts and the same bar under two rolled-up ancestors reads as two pieces
of work. v1 ships `first | error`; `duplicate` is a documented future option if
real demand appears.

**Acceptance Criteria:**

1. A `gantts:` block loads, validates, and appears in `/api/v1/_config`.
*Test:* valid gantt round-trips; each malformed variant (unknown entity_type,
unknown relation in `hierarchy`, property not on the type, non-date property in
a date role) yields a specific load error.
2. Recursive self-referential containment renders to unbounded depth.
*Test:* project ⊃ project ⊃ project ⊃ epic ⊃ ticket → 5-level tree with correct
parent/child links.
3. A parent with no dates of its own gets the min/max envelope of its
transitive descendants. *Test:* exact rolled dates over 3 levels.
4. A child escaping its parent's declared window is a breach with direction
and magnitude. *Test:* after-only, before-only, both.
5. `multi_parent` policies behave as declared. *Test:* a two-parent node
appears once under `first`; fails the load under `error`.
6. A containment cycle never hangs. *Test:* A ⊃ B ⊃ A fails load under
`on_cycle: error`, prunes under `prune`; neither recurses infinitely. Cycle
detection runs on the VISIBLE subtree (RR-4JF5U3).
7. Hidden entities never leak — by row, by field, or by arithmetic.
*Tests:* (a) two-principal differential fold (RR-5KEF8E): a hidden child that is
the max-end descendant must change the parent's `rolled.end`; (b) a row-visible
entity whose date FIELD is `visible:`-hidden contributes nothing to any
ancestor's span (RR-7PK0YW); (c) counts/`truncated` post-filter (RR-Y7MINP).

## Research

- [x] For larger features: run `/research` to create a structured research doc
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A — the interactive prototype
(`mockups/gantt-drilldown.html`) served this role; findings in the ticket body.

**Existing Solutions:**

*Libraries:* third-party Gantt components rejected (frappe-gantt, dhtmlx,
vis-timeline) — none model recursive self-referential containment with
drill-down re-rooting, and all fight the SPA's design tokens. Bar geometry is
~200 lines of maths, proven by the prototype.

*The feature has TWO precedents, one per half (corrected by design review,
RR-UZR2JK):*

**Config half mirrors `calendars:`** — struct/validation/normalization/wire
passthrough:
- `Calendars map[string]Calendar` `config.go:92`; `CalendarSource` `:746` is
the `GanttSource` template (`yaml:"entity_type" json:"entity"`, `Where` list,
per-source property names)
- `validate_calendars.go` (`validateCalendars` `:53`, `NormalizeCalendars`
`:303`); dispatch `validate.go:336`; nav check `:438-443`; normalization call
`app.go:777`
- Wire: `v1.Config.Calendars` = `map[string]dataentryconfig.Calendar`
(`apiwire/v1/responses.go:261`), emitted `api_v1.go:1509`; OpenAPI stub
`openapi/schemas.go:421`

**Endpoint half anchors on `_views`** — the codebase's only server-side
nested-traversal endpoint (`GET /api/v1/_views/{type}/{id}`, `api_v1.go:115`,
`views_handler.go:402`), which already implements:
- entry gate BEFORE traversal (`views_handler.go:427-433`) — hidden id 404s
byte-identically to a missing one (`entityNotFoundTitle`, RR-NGMI)
- three independent bounds: fixpoint cap (`views.go:34-43`), per-rule
`MaxDepth` default 10 (`:91-101`), `visited` cycle guard (`:167-178`)
- recursive walk `traverseViewRecursive` (`views.go:163-176`) — the primitive
to extract a tree-retaining variant from (RR-YCMFES)
- raw-traverse-then-redact-once ordering (`views.go:56-68`) — reused with the
fold-specific amendment below
- one bug NOT to copy: `views.go:105-110` swallows an unparseable `where:` and
continues unfiltered; gantt sources fail closed instead

*Settled precedent for the ACL orphan policy:* `VisibleTracer.rebuild`
(`internal/visibility/tracer.go:104`) already prunes the entire subtree at a
hidden node ("hidden = nonexistent", `tracer.go:17-25`). Drop-not-promote is
consistency with the established traversal semantics, not a novel call.

*Reference interaction:* flame-graph drill-down (click to re-root, breadcrumb,
axis rescale) applied to a time axis.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

*Server-side tree.* The SPA's only relation accessor is per-entity
(`getEntityRelations`, `frontend/src/api/entities.ts:287`) — client-side
recursion is N+1 per level.

GET /api/v1/gantts/{id}   ->  { roots: [GanttNode], truncated: bool }

GanttNode { id, type, title, planned:  {start, end} | null, rolled:   {start,
end} | null, committed: date | null, breach:   {before: bool, after: bool},
children: [GanttNode] }

`planned`/`rolled` ship separate (merging discards the breach). **GanttNode
exposes NO relation properties** — relation meta is unredacted on the visibility
path (RR-4JF5U3, TKT-0RBFN0); an edge-meta field would need
`affordances.RelationFieldVerdicts` first.

*Handler:* a `ganttHandler` struct (RR-SVIJ11 — `App` is at
`//plimsoll:max-methods=104`, hard ceiling), read-only, no `writeMu`, following
`viewsHandler`/`exportHandler` (`app.go:218-232`). Registered strictly under
`/api/v1/`; **fails closed if the ctx read gate is absent** (RR-BXXUC6 —
`readGateFromContext` defaults to a permissive `nopReadGate`; follow
`gatedScriptReader`'s DenyReader-on-fault stance, `app.go:558-580`).

*Traversal:* extract a tree-retaining variant of `traverseViewRecursive`
(RR-YCMFES; both callers share it — cf. the `storeutil` `TopValues` hoist,
`9c87c5c5`). Edges load in bulk: one `store.ListRelations` per `hierarchy`
relation type; endpoint visibility via the shared `PolicyReader.FilterRelations`
— do NOT write a fourth both-endpoints implementation (RR-4JF5U3).

**THE PIPELINE ORDER IS THE SECURITY PROPERTY (RR-5KEF8E, RR-7PK0YW,
RR-Y7MINP):**

1. **Row-gate** the node set (hidden node ⇒ subtree dropped, per
`VisibleTracer` semantics).
2. **Zero hidden date fields**: for each surviving node, blank any date-role
property the principal's field verdicts hide — the fold must never see a
`visible:`-hidden value, or the parent's span launders it into a readable field.
3. **Fold** (post-order roll-up) over what remains. Cycle detection runs here,
on the gated subtree.
4. **Cap + `truncated`** computed on the folded, filtered tree — a dropped
subtree never sets `truncated` (precedent: `TestACLList_PaginationLeakSurfaces`,
`acl_list_test.go:88-91`).
5. **Redact exactly once** at the response boundary (`visibility.Redact` is
non-composable — `policyreader.go:222-227`, RR-Q1VCKR — so no per-level
redaction inside the fold).

*Alternatives rejected:* client-side tree (N+1); a second independent DFS
(RR-YCMFES); a generic tree-view abstraction (speculative); `multi_parent:
duplicate` (incoherent roll-up, cut from v1).

**Files to modify:**

- `internal/dataentryconfig/config.go` — `Gantt`, `GanttSource`;
`Gantts map[string]Gantt` near `:92`; `NavigationEntry.Gantt` near `:845`; gantt
icon in `ValidIconNames` `:337`
- `internal/dataentryconfig/validate.go` — `"gantts": true` `:30`;
`"gantt": "gantts"` hint `:50`; dispatch `:336`; nav arm `:438`
- `internal/dataentryconfig/validate_gantts.go` — new (`validateGantts`,
`NormalizeGantts`; validate `filter_controls`, unlike calendar)
- `internal/dataentry/config.go` — aliases near `:48`
- `internal/dataentry/app.go` — `NormalizeGantts` `:777`; wire `ganttHandler`
near `:218-232`
- `internal/dataentry/gantt_handler.go` — new: struct, route, pipeline steps
1-5
- `internal/dataentry/api_v1.go` — route registration; `Gantts:` in config
literal `:1509`; `resolveGanttDirections` near `:1408` if card fields ship
- `internal/apiwire/v1/responses.go` — `Gantts` map + `GanttNode` wire type
- `internal/openapi/schemas.go` — `"gantts"` stub `:421`
- `internal/dataentry/views_handler.go` — sidebar arm `:359`
(`/gantt/<key>`)
- `frontend/src/types/config.ts`, `stores/schema.ts`, `router/index.ts`,
`utils/icons.ts`, `api/gantts.ts`, `views/GanttView.vue`
- `docs/data-entry.md` — after Calendars (~`:2560`)

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

- *gantt config* — every entity type, relation type, property name checked at
LOAD against the metamodel (date roles must be `PropertyTypeDate`/
`PropertyTypeDatetime`, `metamodel/types.go:427-428`). Unparseable `where:` is a
load error (fail closed — unlike `views.go:105-110`).
- *`{id}` path* — exact map key; unknown → 404 (config names not secret).
- *drill root / depth params* — root passes the same read gate as any node
(denied ⇒ 404 identical to missing, `entityNotFoundTitle`); depth clamped.

**Security-Sensitive Operations:**

- The five-step pipeline above IS the security design; each step cites its
finding. Roll-up is per-principal — **never cached across principals.**
- Orphan policy: drop, not promote (RR-5KEF8E rationale +
`VisibleTracer.rebuild` precedent).
- `permission:` on the NavigationEntry is UX-only (RR-CBSVIF). If the endpoint
itself needs a gate (expense argument — the fold is O(tree)), model it on
`gateDocumentPermission` (`standalone_document_handler.go:41-51`), never
`permitsGatedUIElement` (RR-BXXUC6).
- DoS: `visited` set, max depth, node cap.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**

| AC | Test |
|----|------|
| 1 | `validate_gantts_test.go` — `validGantt()` builder + one-defect-per-case table |
| 2 | handler test, 5-level self-referential fixture |
| 3 | roll-up unit test, exact folded dates over 3 levels |
| 4 | breach table: after / before / both / neither |
| 5 | two-parent fixture under `first` and `error` |
| 6 | cycle fixture under both policies; visible-subtree detection |
| 7 | (a) two-principal differential fold; (b) field-hidden date contributes nothing; (c) post-filter cap/count |
| — | `gantt_sidebar_test.go` mirroring `calendar_sidebar_test.go:61,90` |
| — | fail-closed test: request without ctx gate → denied, not full reads |

**Edge Cases:**

- Empty gantt → empty tree, 200
- Dates-no-children / children-no-dates / neither
- `start` after `end` (bad data): degenerate render, no crash, no swap
- start == end still visible (min width)
- `committed` alone — milestone target
- Depth cap boundary (n, n+1)
- Self-loop
- Diamond under `first`: deterministic single placement
- Dates compared as dates, never instants

**Negative Tests:**

- Unknown gantt id → 404
- Absent relation type in `hierarchy` → load error
- Non-date property in a date role → load error
- Property absent from the type → load error
- `multi_parent: error` with a real multi-parent node → load error naming it
- `multi_parent: duplicate` → load error (not in v1)
- Unparseable source `where:` → load error
- Drill root the principal cannot see → 404 identical to missing

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

- *Fold cost O(nodes)* + `FilterRelations`' N-load amplification
(`luareader.go:36-40`: one `GetEntity` per distinct endpoint, no relation
pushdown). Mitigate: bulk edge load, depth/node caps; if `FilterRelations` cost
bites, fix the shared implementation, don't fork it. Any cache must be
principal-keyed.
- *Strict breach semantics on grouping-only containers* — ship strict; add
per-source `roll_up: derive | strict` only on demonstrated need.
- *Scope `xl`* — three PRs: (1) config+validation, (2) endpoint+fold,
(3) SPA view.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**

- [x] `docs/data-entry.md` — `gantts:` block after Calendars, worked date-role
example
- [x] ~~`docs/metamodel.md`~~ (N/A: no metamodel change)
- [x] ~~`docs/cli-reference.md`~~ (N/A: no CLI change)
- [x] `CLAUDE.md` — record the five-step pipeline (gate → zero hidden fields →
fold → cap → redact-once) as a load-bearing invariant
- [x] ~~`README.md`~~ (N/A: no project-level change)

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:**

| ID | Severity | Summary | Status |
|----|----------|---------|--------|
| RR-5KEF8E | critical | Fold must run post-row-gate | addressed (pipeline step 1→3) |
| RR-7PK0YW | critical | Zero `visible:`-hidden date fields before fold; Redact once at boundary | addressed (steps 2, 5) |
| RR-Y7MINP | critical | Counts/`truncated` post-filter | addressed (step 4) |
| RR-UZR2JK | significant | Endpoint anchors on `_views`, not calendars | addressed (Research/Approach re-anchored) |
| RR-BXXUC6 | significant | Fail closed on missing gate; 403 modelled on documents | addressed (Approach + Security + test) |
| RR-4JF5U3 | significant | Reuse `FilterRelations`; no edge meta in v1; cycles on visible subtree | addressed (Approach + AC6) |
| RR-YCMFES | significant | Reuse `traverseViewRecursive` via tree-retaining variant | addressed |
| RR-CBSVIF | significant | `permission:` is nav-entry UX-only; 403 test was fictional | addressed |
| RR-SVIJ11 | significant | Extracted `ganttHandler` struct (plimsoll) | addressed |
