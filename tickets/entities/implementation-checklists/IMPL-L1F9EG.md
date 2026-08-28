---
id: IMPL-L1F9EG
type: implementation-checklist
title: 'Implementation: lua bounded read API shape (stage 1)'
status: done
---

## Development

- [x] Unit tests written for new code
- [x] Integration tests written (test full flow, not just units)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] Error handling in place (errors surfaced, not swallowed)

## Test Quality

- [x] Using fixture builders or factories for test data
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

Five mutations, each confirmed FAILING before revert:

1. Remove the ceiling clamp → `{limit = 999999}` returned 999999.
2. Ignore unknown option keys → `{cursor = "abc"}` silently accepted.
3. Skip redaction on the pushdown's `AllowAll` branch →
`row TKT-1 leaked a hidden property: map[secret:S title:A]`. This is the
RR-1W1G6K / RR-OXE47R defect caught by a live test rather than a plan note — the
exact #1188 class.
4. Drop the iterator early-stop → `pulled 50 rows for a limit of 10`. BOTH the
row count and the pull count assert, so a slice-after-materialize implementation
cannot pass while doing 5x the store work.
5. Restore the pre-TKT-FVQ4 `break` → truncated list, no error raised.

Fail-closed also mutation-checked: making a scope-composition error fall back to
the non-pushdown path was caught by `TestListPushdown_ScopeErrorFailsClosed`.

End-to-end on the live graph (933 review-responses, 235 tickets):

- `rela.list_entities` returned 933 and 235 — EXACTLY matching `ls | wc -l`,
so nothing truncates at real scale under the 2000 ceiling.
- `scripts/dev-status.lua` runs unchanged.
- 119/120 validation rules pass; the single failure is this ticket's own
in-review merge gate.

Static checks: `just lint` 0 issues, `just lint-md` 0 issues, `just arch-lint`
OK, `just plimsoll` OK. **No new arch-lint rules were needed** — the pushdown is
a consumer-side interface in `visibility`, and the new Lua helpers are package
functions (Runtime is pinned at max-methods=120).

Coverage: `internal/visibility` 89.2%.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — `parseReadOpts` is shared by every read
binding rather than duplicated per binding, which is what stops the three
surfaces drifting again (the drift is why this ticket exists). The pushdown's
optional-capability type assertion follows the existing `store.Formatter`
pattern rather than widening `Reader`.
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind

**A test defect found and fixed during the work:** the clamp subtest initially
PASSED for the wrong reason. `t.Parallel()` subtests resume after the parent
returns, so the deferred ceiling restore ran first and they asserted against the
real 2000 instead of the lowered 10. Made those tests serial and added a setup
assertion inside `swapMaxReadLimit`, so the same mistake now fails at the swap
rather than as a confusing assertion miss. Worth recording because the test was
green while testing nothing.
