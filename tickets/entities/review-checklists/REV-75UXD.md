---
id: REV-75UXD
type: review-checklist
title: 'Review: Detail-view list section items are not clickable (no href, broken router push)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`) — 610 frontend tests, all Go packages green.
- [x] Lint clean (`just lint`) — 0 issues after `golangci-lint` v2 upgrade.
- [x] Coverage maintained (`just coverage-check`) — total 74.9%, package floors satisfied.

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** Two passes of design review (RR-ODSY8 through RR-ZRC26
from pass 1; RR-IW3LV through RR-0AM41 from pass 2; RR-CRIT1/CRIT2 from code
review) plus one code-review pass (CRIT-1, CRIT-2, SIG-1 through SIG-5,
MIN-1..7). All criticals + significants addressed. Minor/nit items deferred with
reasons documented in the corresponding review-response entities.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

- AC1 — list items clickable in custom detail views: **PASS**.
Verified manually with puppeteer against `tickets/`: opened
`/view/feature_detail/FEAT-001`, the "Required Concepts" section rendered `<a
class="list-link">` elements with real `href`s pointing at
`/view/concept_detail/<entity-id>`. Click navigated to the concept detail view
as expected.
- AC2 — back-button returns to originating view: **PASS**. After clicking,
the EntityView page showed a back button as `<a
href="/view/feature_detail/FEAT-001">← Back</a>`, sourced from the `?return_to=`
query.
- AC3 — `entity_views.<type>.detail_view` is the source of truth: **PASS**.
Migration moved `tickets/data-entry.yaml` (4 types) and
`prototypes/data-entry/project/data-entry.yaml` (2 types). API config response
includes the new `entity_views` map.
- AC4 — keyboard a11y: **PASS** by inspection. `:focus-visible` rule is
compiled into the SPA bundle (scoped `.list-link[data-v-…]:focus-visible`).
Keyboard tab triggers it; programmatic `.focus()` does not (browser-correct
behavior).
- AC5 — migration is idempotent: **PASS**. Test
`TestDetailViewToEntityViewsMigration_Idempotent` exercises a migrate-able +
conflict mix, runs Apply twice, asserts second pass is no-op.

## Documentation

N/A — bug fix; no user-facing documentation needed.

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI
- [x] All CI checks pass

**PR:** https://github.com/sourcehaven-bv/rela/pull/649
