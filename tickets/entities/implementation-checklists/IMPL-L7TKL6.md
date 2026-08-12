---
id: IMPL-L7TKL6
type: implementation-checklist
title: 'Implementation: Render list cells and kanban card fields through the widget registry'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Code implemented per the planning-checklist approach
- [x] Follows existing patterns (`PropertyDisplay.vue`'s precomputed-rows +
`<component :is>` + sibling ACL `v-if`)
- [x] No unrelated changes in the diff
- [x] Commit messages explain the why

Commits: `277b9371` (migration), `1c0841c6` (review fixes).

## Quality Checks

- [x] `npm run test:run` — 103 files, 1649 tests pass (was 1616 before; +33)
- [x] `npm run typecheck` — clean
- [x] `npm run lint` — 0 errors. Remaining warnings are pre-existing
`max-lines` on `EntityList.vue` / `KanbanView.vue`, both already well over 500
lines before this change.
- [x] `npm run build` — succeeds
- [x] ~~`just coverage-check`~~ (N/A: Go package-floor thresholds only; the
frontend has no coverage enforcement — `frontend/CLAUDE.md` and the root
Coverage section both state unit tests run plain)

## Acceptance Criteria Evidence

1. **Per-type rendering matches the pre-migration string** — PASS. Verified by
rendering each case through a mounted `EntityList` and comparing against
`formatCellValue` directly: list-date `"2026-01-01, 2026-02-02"`, list-rrule
`"every day"`, empty-enum-list `""`, bool-false `"No"`, int-zero `"0"`,
scalar-date `"Mar 4, 2026"`, empty-string-list `""` — all identical. The one
intended difference is a non-empty enum list, which badges (two `.badge`
elements) instead of joining, as it already did.
2. **No console warnings on any built-in type** — PASS.
`EntityList.cells.test.ts` renders all eight types and asserts `console.warn` is
never called.
3. **Resolution once per column** — PASS. 50 rows × 3 columns produces exactly
3 `resolveFromHint` calls (spy assertion).
4. **Relation / ACL / emptiness unchanged** — PASS. Relation cell renders joined
titles; inaccessible cell renders the lock and never reaches a widget; mobile
hides an empty column and keeps a `false` boolean.

## Bug fix verification

The kanban fix was verified negatively: reverting `getCardFieldValue` and
`visibleCardFields` to their previous form makes exactly the three bug-fix tests
fail (`false` dropped, `0` dropped, `true` rendered instead of `Yes`), and
restoring the fix makes them pass. A test that would also pass against the old
code would prove nothing.

## Edge cases covered

`null`/`undefined`/`''`/`[]`, `false`, `0`, list-valued variants of every scalar
type, a property with no schema entry, and both deliberate routings (boolean →
text, file → text) pinned so a future "simplification" to `resolve(propertyDef)`
fails loudly.

## Known gaps

- Schema/data mismatch on a scalar-declared enum holding an array still warns
per row and truncates to the first element (RR-XEC2RD, deferred — pre-existing
in `SelectWidget`, shared with detail views).
- The four render sites still duplicate their structure; no shared cell
component was extracted. All four are now covered by tests.
