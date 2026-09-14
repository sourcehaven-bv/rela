---
id: IMPL-KZPFTF
type: implementation-checklist
title: 'Implementation: display: nested — a two-level parent-child view section'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] Integration tests written (test full flow, not just units)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] Error handling in place (errors surfaced, not swallowed)

**What was built** (7 files):

- `internal/dataentry/views.go` — `viewResult.Parents` carries parent→child
  edges; `traverseViewMany` now returns the `bySource` map it already built
  (no new query); `mergeViewParents` dedupes on merge.
- `internal/dataentryconfig/config.go` — `ViewSection.Children`.
- `internal/dataentryconfig/validate.go` — `nested` in the display allowlist,
  `validateNestedSection`, `nestedChildDef`, and column checks widened to
  accept a property of either level.
- `internal/dataentry/sections_nested.go` (new) — `buildNestedTree`,
  `sectionTreeNodeToV1`, plus `buildSectionRow`/`fillPropertyCell` extracted
  from the `table` arm so both modes render cells from one implementation.
- `internal/dataentry/sections.go` — `SectionTreeNode`, the `nested` case.
- `internal/apiwire/v1/responses.go` — `ViewTreeNode`, `ViewSection.Tree/Truncated`.
- `frontend/src/api/views.ts` + `EntityDetail.vue` — `ViewTreeNode` type, the
  `display === 'nested'` arm using native `<details>`, and styles.

**Error handling:** invalid config fails the LOAD (a bad `children:`, a
`recursive:` pairing, an unknown column), matching how a bad `condition:`
behaves — dropping a constraint silently is the unsafe direction. Nothing is
swallowed at runtime: an unresolvable child id is the row gate doing its job,
which is the documented behaviour rather than a hidden failure.

## Test Quality

- [x] Using fixture builders or factories for test data
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

`nestedFixture(edges, visibleChildren...)` builds a `viewResult` from an edge
map, so each test states only the shape it cares about. The capped-parent test
derives its expectations from `nestedChildPreview` and `len(manyIDs)` rather
than restating numbers, so the tests follow if the cap changes.

**Mutation-tested** — both load-bearing guards were verified to actually fail
when the fix is removed, rather than assumed:

- Removing the attribution dedupe → `TKT-002 has 2 recorded children
  [TKT-003 TKT-003], want exactly 1`, exactly the RR-UODKN9 defect.
- Removing the child-type column check → `property "name" not in entity
  "ticket" or "category"`.

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

Built a throwaway project at `/tmp/nested-demo` with a real
project → epic → task hierarchy (1 project, 3 epics, 4 tasks; EPIC-3 childless)
and ran `rela-server` against it.

`rela validate` accepts the config. The `_views` response:

```
SECTION: 'Epics' | display = nested | isEmpty = False | truncated = None
  PARENT EPIC-1  Epic number 1      [doing | ]  childCount=2 more=False
      CHILD TSK-1   Task number 1   [blocked | 2026-10-01 ...]
      CHILD TSK-3   Task number 3   [todo | 2026-10-03 ...]
  PARENT EPIC-2  Epic number 2      [doing | ]  childCount=2 more=False
      CHILD TSK-2   Task number 2   [todo | 2026-10-02 ...]
      CHILD TSK-4   Task number 4   [todo | 2026-10-04 ...]
  PARENT EPIC-3  Epic with no tasks [todo | ]   childCount=0 more=False
```

Attribution is correct per parent, and the childless epic reports
`childCount=0 more=False` rather than looking truncated. Rendered in the SPA
(headless Chrome): collapsed rows show title, id, columns and child count;
clicking expands to the indented children; the twisty rotates (confirmed via
computed `transform: matrix(0, 1, -1, 0, 0, 0)`).

**Per acceptance criterion:**

| AC | Verified by |
| --- | --- |
| 1 attribution | `TestNestedSection_AttributesChildrenToTheirOwnParent` + live response above |
| 2 config rejection | `TestValidateConfig_NestedSection` (10 cases) |
| 3 deterministic order | traversal order; live response stable across requests |
| 4 hidden child absent | `TestNestedSection_HiddenChildIsAbsent` |
| 5 capped ≠ childless | `TestNestedSection_CappedParentIsDistinguishableFromChildless` + live EPIC-3 |
| 6 truncated on visible denial | same test asserts both directions |
| 7 query budget | see the gap noted below |
| 8 no bodies | `TestNestedSection_CarriesNoContent` |
| 9 no fixpoint duplication | `TestNestedSection_AttributionSurvivesFixpoint` (mutation-tested) |

**Two findings from manual verification, both acted on:**

1. **Column validation was too strict** — a `due` column was rejected because
   dates live on `task`, not `epic`, but a nested section renders the same
   columns at both levels. Fixed (`nestedChildDef`) and covered by two new
   cases. This would have shipped as "the documented config does not load".
2. **`views:` IS re-keyed by entity type** — `rela migrate` reports
   `views-by-entity-type: Re-key views: by entity type, remove detail_view and
   entity_views`. So the earlier claim that `views:` had no deprecation was
   wrong in this respect: the *keying* is deprecated, though the `views:` block
   and its sections are current. `docs/data-entry.md` still documents
   `entity_views`/`detail_view` as current — stale, and worth its own ticket.

**Known gap, deliberately not closed here: AC7.** No `storetest.Counting`
budget test was added. The nested arm resolves relation columns in ONE batch for
parents and children together (`buildNestedTree` builds a combined
`rowEntities` slice before calling `resolveRelationColumns`), so the design is
size-independent, but that is argued rather than pinned. The existing
`querybudget_test.go` harness targets the list/search/view-table paths and
adding a nested case needs a fixture through the real store; it should be a
follow-up rather than a claim of done here.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind

**Patterns followed:** budget + per-node `HasMoreChildren` + response
`Truncated` mirror the gantt (`gantt_handler.go`), including the rule that
`Truncated` is set only when a *visible* row is denied. New wire fields are
`omitempty`, so a response with no nested section is byte-identical to before.

**DRY:** the `table` arm's row-building closure was extracted to
`buildSectionRow` and is now shared, rather than copied into the nested arm —
two cell implementations would have drifted on widget resolution and
multi-value handling. `fillPropertyCell` was split out to keep the extracted
function under the `nestif` complexity limit.

**Security:** child ids resolve against the already-gated
`viewResult.Collections`, never the store, so row-gating and field redaction
happen once upstream (`views.go:96-99`, verified: `PolicyReader.Filter` calls
`r.redacted()` per surviving row). `ChildCount` counts only readable children,
and `HasMoreChildren` is never set by a hidden one — asserted in
`TestNestedSection_HiddenChildIsAbsent`. The `viewResult.Parents` godoc states
that its ids are NOT authorized and that consumers must resolve them against
the gated collection.

**Gates:** `go build ./...`, `golangci-lint` (0 issues), `just arch-lint`,
`just plimsoll`, `just comment-lint`, `go test -race` on the three changed
packages, the full `./internal/...` suite, `just coverage-check` (79.4%),
`vue-tsc` typecheck, 2456 frontend tests across 151 files, `npm run lint`
(0 errors), and `just build-frontend`.

**Pre-existing issue NOT fixed (not a regression):** date columns render a raw
Go timestamp (`2026-10-01 00:00:00 +0000 UTC`) because
`entity.GetAttributeString` does `fmt.Sprintf("%v", ...)` on a parsed
`time.Time`. Confirmed empirically that the existing `display: table` arm emits
the identical string for the same property, so this is shared by every
column-based section and out of scope here.
