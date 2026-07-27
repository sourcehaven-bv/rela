---
id: RR-W7L33A
type: review-response
title: 'Minor cleanups: publish() is a passthrough, HistoryView duplicates its defaults inline, e2e waiter names collide'
finding: 'Three small consistency issues. (a) publish() (useVersionSelectionSync.ts:155-157) is a one-line passthrough to writeToQuery() whose docstring implies it does something extra ("without recomputing") — writeToQuery never recomputed. (b) HistoryView.vue:109-110 repeats the default pair inline in the restore branch, duplicating the expression already in defaults() at :45-46; the sibling RelationHistoryView extracted defaultSelection() precisely to avoid this. Both views'' restore branches also poke the refs directly, bypassing the composable''s write path — a resetToDefaults() on the composable would close that. (c) e2e naming: relation-history.page.ts now has both waitForTimeline (exact count) and waitForTimelineAtLeast (poll), while history.page.ts names its POLL waitForTimeline — so the same method name means different things on the two page objects.'
severity: minor
resolution: All three fixed. (a) publish() removed; the composable now exports writeToQuery directly as `publish`, with the 'does not recompute — select owns that' note folded into writeToQuery's own docstring, so there is no passthrough to check. (b) HistoryView gained defaultSelection(), mirroring the sibling view, so the default pair exists once; both views' restore branches now call the new resetToDefaults() on the composable instead of assigning refs directly, closing the last write path that bypassed it. Two unit tests cover resetToDefaults (ignores the URL; is side-effect free). (c) history.page.ts's poll waiter renamed waitForTimeline → waitForTimelineAtLeast to match the relation page object's semantics, with a comment noting why; all call sites updated.
status: addressed
---

## Finding

Three unrelated minor issues, grouped because each is a small consistency fix.

1. **`publish()` adds nothing** over `writeToQuery()` and its docstring implies
otherwise, forcing the reader to check.
2. **`HistoryView` duplicates its default pair** inline in the restore branch,
while the sibling view extracted `defaultSelection()` for exactly this reason.
Both restore branches also assign refs directly, bypassing the composable's own
write path.
3. **E2E waiter names collide across page objects** — `waitForTimeline` means
"exact count" on one and "at least" on the other.

None of these can produce a wrong result today; they're all "someone picks the
wrong one later" risks.
