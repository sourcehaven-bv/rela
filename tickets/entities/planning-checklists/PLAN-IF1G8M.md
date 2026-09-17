---
id: PLAN-IF1G8M
type: planning-checklist
title: 'Planning: display: nested — a two-level parent-child view section'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

IN: a `display: nested` view section rendering expandable parent rows;
parent→child attribution retained through traversal; a nesting shape on the
section wire type; node budget + truncation signalling; one SPA dispatch arm.

OUT, both descoped by the user so this ships as the nested view alone:

- **any per-parent rollup or aggregate (TKT-ZAD9PS)** — removes the ACL-fold and
  enum-validation risk entirely.
- **child sorting, including a `Sort` field on `ViewSection` (TKT-9OFGH4)** —
  the plumbing would be built against a comparison rule that is about to change,
  and could not yet produce the wanted order. Children render in traversal order.

Also out: more than two levels; auto-emission on the generated detail page
(`buildDefaultViewConfig`); a standalone route or sidebar entry; any write
affordance.

**Acceptance Criteria:** as listed on TKT-MJKZQ3 (8 criteria). Test scenarios
mapped one-for-one under Test Plan below.

## Research

- [x] For larger features: run `/research` to create a structured research doc
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A — the design space was explored directly against the
codebase and six static mockups in `mockups/project-overview/`, with the four
open design questions settled by the user (detection, bounding, rollup
semantics — since descoped to TKT-ZAD9PS — and config home). A RES entity would restate that without adding to it.

**Existing Solutions:**

No library applies: this is a server-side aggregate over an ACL-gated graph plus
a section renderer, both rela-specific.

Reusable in-tree patterns found:

- **Gantt** (`internal/dataentry/gantt_handler.go`) is the closest sibling and
  the source of three decisions: the documented gate→redact→fold→cap order
  (:29-33), `ganttBudget{remaining, truncated}` accounting (:625-639), and
  `GanttNode.HasMoreChildren` (`internal/apiwire/v1/responses.go:1223`) for
  distinguishing a capped parent from a childless one.

  Exact pipeline to mirror, from `handleV1Gantt` (:95): `buildGanttForest`
  (:176) → `loadGanttNodes` (:252) → `loadGanttType` (:311), where the row gate
  is `ganttReadVerdict` (:280) and redaction is `visibility.RedactHeader` (:328)
  or `visibility.Redact` (:339) **exactly once per entity**; `where:` filters run
  post-redact in `addGanttNodes` (:349); then `finishGanttForest` (:192) →
  `linkGanttParents` (:581) → `foldGantt` (:676, post-order); finally the cap via
  `emitGanttNode` (:714) with `budget.take()` (:632) and
  `resp.Truncated = budget.truncated` (:151). Invariant: never `Redact`
  already-redacted input.
- **Gantt `hierarchy:`** (`internal/dataentryconfig/config.go:951-956`) settles
  that containment is *declared*, not inferred — its doc comment names exactly
  this case ("project contains project, project has-epic, epic has-ticket") and
  makes the list required.
- **Kanban** (`config.go:768`, `:784-788`) precedent for naming a grouping
  property explicitly (`column_property`) and for carrying `value`/`label`/`icon`
  with **no colour** — hence colours come from the frontend palette.
- **Existing section pipeline**: `views.go:96-99` already row-gates and
  field-redacts every collection ONCE before any section builder runs.
- **Query budget tests**: `internal/dataentry/querybudget_test.go` (`readsFor`
  at :136, `assertBudget` at :149) is the harness for AC7.

Measured and rejected (recorded on the ticket): inferring containment from the
metamodel. "Target type has any outgoing relation" yields 103 nested sections on
the real 47-relation `tickets/schema.yaml` (20 on a bug page); `max_incoming: 1`
yields 0 matches in both live schemas because the key is never used.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

The central finding: `viewResult.Collections` is a **flat `map[string][]*entity.Entity`**
(`internal/dataentry/views.go:14-25`). `epics` and `tasks` are two independent
buckets with no record of which task belongs to which epic — `traverseViewMany`
builds exactly that mapping in `bySource` (`views.go:210-221`, from ONE batched
`store.RelationQuery{EntityIDs: sourceIDs}`) and then discards it by flattening
to `out` at :223-227.

So the work is to **retain a mapping that is already computed**, not to issue new
queries. Four steps:

1. **Config** (`internal/dataentryconfig`): add `Children` to `ViewSection`
   (`config.go:1301-1311`); add `nested`
   to `validSectionDisplayModes` (`validate.go:87-94`, enforced at exactly one
   site, :1470). Validation lives in `validateViews` as free-function checks —
   the package has no `Validate()` methods, and RR-4ICH8M's constraint is that a
   metamodel-dependent check cannot be a self-contained field validator, not
   that it needs a new file.

   **`children:` validation.** Reuse the `collections` map that
   `validateViews` already builds for `source:` (`validate.go:1457-1467`, map
   built :1383-1451) — an unknown bucket is a hard error. Also reject
   `recursive: true` on a traverse rule feeding a nested section (see step 2).
   Note `determineTargetType` (:1564-1591) returns `""` for a multi-`to:`
   relation, so the child TYPE is not always statically known. That costs nothing
   here (no `rollup:`/`sort:` property to check against it) but is why TKT-ZAD9PS
   and TKT-9OFGH4 each need a policy for the undeterminable case.

   **Beware the second bucket map.** `viewCollectionTypes` (`validate.go:1176`)
   reimplements the bucket map and its godoc records that it already diverged
   once by omitting the implicit `entry` bucket, silently skipping every
   `source: entry` section. A `children:` check added only to the inline map in
   `validateViews` would half-validate.

   **`max_nodes: 0` is indistinguishable from absent** — `NormalizeGantts`
   (`validate_gantts.go:285-304`) treats `== 0` as unset, so a nested budget of
   0 would silently become the default. Copy gantt's negative check (:134-136)
   and its effective-value coherence check (:138-153), and decide explicitly
   what 0 means.

2. **Traversal attribution** (`internal/dataentry/views.go`): have
   `applyViewTraverse` retain `bySource` per rule — a
   `map[string]map[string][]string` keyed by `collect_as`, i.e. parent id →
   child ids. Carry it on `viewResult` beside `Collections`. No extra store
   read: the data already exists at :221 and is thrown away.

   **It must dedupe on merge (RR-UODKN9).** `executeView` runs every traverse
   rule up to **10 times** in a fixpoint loop (`views.go:65-75`). Collections
   survive that by deduping by id when merging (`views.go:174-188`); a naively
   appended attribution map would list each child once per pass and duplicate
   every child row. Mirror the existing dedupe, and test with a traverse that
   needs more than one pass.

   **`recursive: true` has no attribution (RR-UODKN9).**
   `traverseViewBreadthFirst` (`views.go:233+`) returns a flat id list per level
   and builds no `bySource`. So `recursive: true` + `display: nested` would
   silently lose parent→child mapping. **Decision: reject the combination at
   config load** — this ticket renders exactly two levels, so `recursive` has no
   meaning for it, and a load error is better than a silently flat section.
   Building attribution into the BFS path belongs with a future multi-level
   ticket.

   *Gating comes free.* Because `views.go:96-99` filters and redacts every
   collection once before section building, a nested builder that resolves child
   ids **against the already-filtered `Collections[children]`** inherits
   its gating by construction. Ids in the retained mapping that are absent from
   the filtered collection are exactly the hidden children, and dropping them is
   the gate. This satisfies AC4 without new ACL code — the property to preserve,
   and to pin with a test, is that child resolution reads the filtered collection
   and never the raw store. It is also the precondition for TKT-ZAD9PS.

3. **Section build + wire** (`sections.go:373`, `views_handler.go:562-635`,
   `internal/apiwire/v1`): add a `nested` arm producing parent rows each carrying
   `children[]` and `has_more_children`. Apply a shared node budget, mirroring
   `ganttBudget`, with `truncated` set only when a *visible* node was denied
   emission.

4. **SPA** (`EntityDetail.vue`): one `v-else-if="section.display === 'nested'"`
   arm inserted between :2143 and :2144 (after the table arm, before
   `</section>`), rendering expandable rows. Reuse `ganttLayout.ts`'s
   `flattenRows`/`isRowExpanded` rather than a new tree component. Widen the
   closed `display` union at `frontend/src/api/views.ts:118`.

**Alternatives considered and rejected:**

- *Extend the generated detail page* (`buildDefaultViewConfig`) — rejected
  because defining a view already replaces the generated page
  (`views_handler.go:505-509`, either/or with no merge), and `views:` sections
  are already the layout system (`fields[].span` is the side-by-side field
  mechanism). Adding a merge path would be a second way to do layout. Left open
  as a follow-up; both paths share the executor.
- *Per-parent child paging* — rejected for v1; `_views` has no paging vocabulary
  and adding one is larger than this feature. Node budget instead.
- *Re-query children per parent* to get attribution — rejected: that is the N+1
  the batched traverse exists to avoid, and would fail AC7.

**Files to modify** (line refs verified against the tree):

- `internal/dataentryconfig/config.go:1301-1311` — add keys to `ViewSection`
- `internal/dataentryconfig/validate.go` — there is **one** display allowlist,
  `validSectionDisplayModes` (:87-94), enforced at **one** site (:1470); none in
  `config.go`. `nested` is one mandatory addition there, plus new checks in the
  sections loop of `validateViews` (:1455+). Do **not** add `nested` to
  `sectionDisplayModesRenderingFields` (:117-121) — this section uses `columns:`,
  not `fields:`, so adding it would suppress a legitimate inert-`render:` warning.
- `internal/dataentry/views.go` — retain `bySource` per rule on `viewResult` (:16)
- `internal/dataentry/sections.go:373` — the new `case "nested":` arm, in the
  COLLECTION-path switch (:302). **Not** `views_handler.go` — that is only the
  call site (:536). Keep `executeSidePanel` (:419) in sync.
- `internal/dataentry/views_handler.go:562-635` — `SectionData` → apiwire conversion
- `internal/apiwire/v1/responses.go:915-928` — add `Tree []ViewTreeNode`
  (`omitempty`) to `ViewSection` plus the new node type near `ViewRow` (:1000)
- `frontend/src/api/views.ts:118` — **widen the closed `display` union**; it is
  currently `'properties' | 'content' | 'table' | 'cards' | 'list'`
- `frontend/src/components/entity/EntityDetail.vue` — new arm between :2143 and
  :2144 (after the table arm, before `</section>`)
- `docs/data-entry.md` — the `## Views` section (:1220) and its display table

**Corrections to earlier assumptions** (found while mapping the code):

- `source:` is **already** validated against real traverse buckets
  (`validate.go:1457-1467`, against a `collections` map built at :1383-1451). So
  AC2's `children:` check should reuse that same map rather than invent a lookup.
- **Second-level buckets are already legal and tested** — `from: epics` validates
  because `from:` is checked against the *accumulated* collections map
  (`validate.go:1391-1400`), pinned by `validate_test.go:1449-1483` and
  `views_test.go:163,213`. No traverse change needed for the two-step walk.
- `columns:` are already validated against the metamodel when the source type is
  known (`validate.go:1519-1535`), which resolves the RR-4ICH8M concern about
  where metamodel-dependent checks may live: section-level checks already have
  metamodel access, so a future `sort:`/`rollup:` property check has a home.
- An unrecognised `display` **silently produces an empty section** — there is no
  default arm in the switch (`breakdown` is allowlisted but has no arm). So the
  config allowlist is the only guard; forgetting it means a silent blank section,
  not an error. Worth an explicit test.
- **`section.isEmpty` short-circuits the whole ladder** at `EntityDetail.vue:1720`,
  so a nested section with zero parents never reaches its own arm. `IsEmpty` must
  be set on the parent list, and the empty-state message is the generic one.
- **Reuse `frontend/src/utils/ganttLayout.ts`** rather than writing a tree
  component: `flattenRows(roots, expanded, defaultDepth)` (:233), `FlatRow`
  (:16), `isRowExpanded` (:262) — pure, already unit-tested
  (`ganttLayout.test.ts`), and consumed by `GanttView.vue`. No generic
  Tree/ExpandableRow component exists in `frontend/src/components/`.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

- `display:`, `children:` — operator config from `data-entry.yaml`.
  Validated at config LOAD against an allowlist (`display`) and against the
  metamodel (`children` must name a real traverse bucket). Invalid config fails
  the load, consistent with
  how a bad `condition:` is a load error: dropping a constraint silently is the
  unsafe direction.
- Entity ids reaching the child resolution come from the traversal, not the
  request; the request carries only entity type + id, already gated at
  `views_handler.go:501` before the pipeline runs.
- Per `CLAUDE.md`, config *contents* are not confidential — a section name or
  property name needs no concealment. No per-principal filtering of config keys.

**Security-Sensitive Operations:**

1. **Children must resolve against the gated collection, never the store.**
   `PolicyReader.Filter` calls `r.redacted()` on each surviving row
   (`internal/visibility/policyreader.go:88-93`), so every `Collections` member is
   row-gated AND field-redacted before a section builder runs (`views.go:96-99`).
   Resolve child ids against that filtered slice: ids in the attribution map but
   absent from it are exactly the hidden children, and dropping them IS the gate.
   Verified, not assumed. Pin it with a test — it is also the precondition that
   makes TKT-ZAD9PS's rollup safe to add later.

2. **Truncation flags are an existence oracle if computed pre-filter.** `truncated`
   and `has_more_children` must be computed on the gated tree, and `truncated`
   set only when a *visible* node was denied emission — copying gantt's rule so
   it cannot reveal how close a principal's tree sits to the cap.
3. **Per-principal, never cached across principals.** The child set is a function
   of what this caller may see.
4. Error handling: config errors name the offending key (operator-facing, not
   principal-facing). Runtime faults use the existing `writeGateError` posture; a
   denied entry is already indistinguishable from a missing one at :501.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios (AC → test):**

1. *Renders parents + children* — handler test: a view with `display: nested`
   over project→epic→task; assert section carries 2 parents, each with its own
   children, correctly attributed (epic A's tasks not under epic B).
2. *Config rejection* — table-driven config tests: `children:` naming an unknown
   bucket → load error; `recursive: true` on a nested section's traverse → load
   error; valid config → loads.
3. *Deterministic order* — two identical requests return children in the same
   order (traversal order). No `sort:` in this ticket, so the test pins
   stability, not a chosen ordering.
4. *Hidden child excluded* — the load-bearing ACL test. Two principals, same
   project; a child hidden from one. Assert the restricted principal does not see
   the row, and that child resolution read the filtered collection rather than the
   store.
5. *Capped parent ≠ childless parent* — set a budget of 2; assert an over-budget
   parent reports `has_more_children: true` while a genuinely childless parent
   reports absent/false.
6. *`truncated` only on visible denial* — mirror
   `TestGantt_TruncatedIsPostFilter`: a principal whose tree fits within budget
   after gating gets `truncated: false` even when the raw tree exceeds it.
7. *Query budget* — extend `internal/dataentry/querybudget_test.go` with a
   nested-section case through `readsFor`/`assertBudget` at 10 and 50 parents;
   equal counts, and pin the constant.
8. *No bodies* — assert the `nested` arm leaves `Content`/`HasContent` unset on
   child rows. **`include_content` does not exist on the `_views` path** (it is
   a list-path parameter in `rowcontent.go`); section content is opt-in by
   `display`, and only the `content`/`cards` arm sets it (`sections.go:373`)
   while `buildSectionEntityData` (:234) sets none. An earlier revision of this
   scenario asserted a parameter that is not there.

One invariant test not tied to a single AC, from design review:

9. *No double-counting under the fixpoint* — a traverse needing more than one
   pass (self-referential containment fixture) yields each child once per parent
   (RR-UODKN9).

**Integration approach:** the handler tests above go through the real
`_views` endpoint with a memstore, which exercises config → traverse →
gate/redact → section build → wire in one path. That is where attribution and
gating interact, so unit tests on the fold alone would not be sufficient.

**Tests to extend rather than create** (verified to exist):

- `internal/dataentry/sections_test.go` — main `buildSections` suite;
  `sections_span_test.go:50` calls `app.views.buildSections(...)` directly and is
  the best template for a new display arm.
- `internal/dataentry/views_test.go` — traverse/`collect_as`, already covers
  second-level buckets at :163 and :213.
- `internal/dataentry/acl_views_test.go` — where AC4 (hidden child excluded) belongs.
- `internal/dataentry/querybudget_test.go` — AC7, via `readsFor` (:136) /
  `assertBudget` (:149).
- `internal/dataentryconfig/validate_test.go` — display allowlist and
  source/collect_as checks (:1044-1146, :1449-1483) for AC2.
- Shared fixtures: `internal/dataentry/test_helpers_test.go`.
- Frontend: `EntityDetail.world.test.ts` / `sectionEditFields.test.ts` are the
  only EntityDetail-adjacent specs (no generic section-render spec exists);
  `frontend/src/utils/ganttLayout.test.ts` if a shared flatten helper is extracted.

**Edge Cases:**

- Parent with zero children (section not empty, but that row has no expansion).
- **Child appearing under two parents — DECIDED: render under both, count under
  both.** Gantt refuses `multi_parent: duplicate` because its fold is
  hierarchical: the same bar under two ancestors double-counts every span above
  it. This section has no fold at all, and TKT-ZAD9PS's rollup would be flat and
  per-parent with no grand-total to corrupt. Two epics genuinely sharing a task
  each legitimately show it. No `multi_parent` knob; document the behaviour.
  (Revisit only if a future ticket adds a section-level total, which WOULD
  reintroduce gantt's problem.)
- `children:` bucket empty for every parent.
- Cycle in the containment relation (project→project) — the traverse depth cap
  (10, `views.go:129`) bounds it; assert no hang.
- Budget exhausted on the FIRST parent — later parents get nothing; acceptable
  under a shared budget (noted as a known trade when this option was chosen).

**Negative Tests:**

- `display: nested` with no `children:` → load error naming the section.
- `children:` pointing at the same bucket as `source:` → load error.
- Malformed budget config (negative) → load error, matching gantt's
  `MaxNodes < 0` check.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

1. **Attribution touches shared traversal code** (`views.go`), used by every
   view. *Mitigation:* additive — retain a map already built at :221 and carry it
   on `viewResult`; no change to what `Collections` contains, so existing
   sections are unaffected. Existing view tests are the regression net.
2. **Wire-shape churn.** `ViewSection` is consumed by the SPA and is a public
   API surface. *Mitigation:* new fields are `omitempty`, so existing consumers
   are byte-identical for non-nested sections.
3. **ACL fold is the highest-consequence part.** *Mitigation:* fold over the
   already-filtered collection so gating is structural rather than a step that
   can be forgotten; AC4/AC6 tests; `/design-review` before implementation.
4. **Shared budget starvation** — one huge first parent can consume the budget.
   Accepted when choosing the gantt pattern; revisit if real usage shows it.
5. **Bar/row order disagreement** until TKT-9OFGH4 lands (bar in declared enum
   order, rows alphabetical). Documented on the ticket; tolerable because the
   bar reads as a distribution, not a sequence.

**Effort:** `l` — backend config + traversal + wire + fold + budget, one SPA arm,
docs. Sized at `l` rather than `m` because the shared traversal change and the
ACL fold both need careful tests; `xl` would imply new endpoint vocabulary,
which the node-budget decision explicitly avoided.

## Documentation Planning

For enhancements: identify what documentation needs updating.

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**

- [x] `docs/data-entry.md` — the `## Views` section (:1220) gains `display: nested`
      with `children:` and the truncation behaviour. The display allowlist table
      needs the new value. State plainly that children are unsorted and that
      sorting is TKT-9OFGH4.
- [x] ~~`docs/metamodel.md`~~ (N/A: no metamodel change)
- [x] ~~`docs/cli-reference.md`~~ (N/A: no CLI change)
- [x] ~~`CLAUDE.md`~~ (N/A for now: one candidate line if the gate-before-fold-over-filtered-collection property proves subtle enough to pin as a rule; decide at code review, not up front)
- [x] ~~`README.md`~~ (N/A: no project-level change)

Also worth a separate follow-up (not this ticket): `docs/customisation.md:206`
claims `<rela-slot>` is "not yet emitted", but it is live in
`NextActionCard.vue:51` and `StatusBar.vue:156`. Stale doc line found while
researching this.

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan (RR-0C3IZX, RR-UODKN9 addressed; RR-P751MM, RR-HQMQJB moot once the rollup was descoped to TKT-ZAD9PS)

**Design Review Findings:**

- **RR-0C3IZX** (critical, addressed) — the plan claimed section `sort:` already
  existed. `ViewSection` has no `Sort` field and the view path does not sort at
  all; the cited fields belong to `List` and dashboard cards. Sorting is new work
  and has been moved to TKT-9OFGH4, which owns the comparison rule it depends on.
- **RR-UODKN9** (significant, addressed) — attribution map must dedupe on merge
  because traverse rules run up to 10× in a fixpoint loop; and `recursive: true`
  has no attribution in the BFS path, now rejected at config load.
- **RR-P751MM** (significant, wont-fix — moot) — redacted rollup values must
  still be counted. Carried onto TKT-ZAD9PS with the rollup.
- **RR-HQMQJB** (significant, wont-fix — moot) — `rollup:` validation is
  three-way because `determineTargetType` returns `""` for a multi-`to:`
  relation. Carried onto TKT-ZAD9PS with the rollup.

Also corrected without a separate response: AC8 cited `include_content=true`,
which does not exist on the `_views` path; and the plan claimed two display
allowlists, one in `config.go`, when there is one, in `validate.go`, at one
enforcement site.

Two claims in the plan were verified as CORRECT rather than taken on trust:

- `PolicyReader.Filter` does field-redact each surviving row
  (`policyreader.go:88-93`), so collections reaching a section builder are both
  row-gated and field-redacted — "gating comes free" holds, and the nested arm
  must keep reading `Collections` rather than the store.
- Second-level traverse buckets (`from: epics`) are already legal, validated and
  tested; no traverse change is needed for the two-step walk.

Decisions taken during review that were previously deferred:

- **Multi-parent: render under both parents**, with no `multi_parent` knob.
  Gantt forbids duplication because its fold is hierarchical and double-counts
  ancestors; this section has no fold, so a genuinely shared child legitimately
  appears under each parent.
- **`recursive: true` + `nested` is a config-load error** rather than a silently
  flat section.
