---
id: REV-AK5QFF
type: review-checklist
title: 'Review: lua bounded read API shape (stage 1)'
status: done
---

## Automated Checks

- [x] All tests pass — CI `Test`, `Fuzz` and `Postgres Backend` green; the new
`internal/lua` and `internal/visibility` suites add 618 lines of tests
- [x] Lint clean — `just lint` 0 issues, `just lint-md` 0 issues, `just arch-lint`
OK, `just plimsoll` OK, all re-confirmed in CI
- [x] Coverage maintained — package floors hold; every new file ships with a
paired `_test.go` (`readopts_test.go`, `pushdown_test.go`,
`list_entities_bound_test.go`)

## Code Review

- [x] Run `/code-review` — six findings raised and recorded as review-responses
- [x] All critical review-responses addressed — RR-1W1G6K (GraphQuery pushdown
would drop field-level redaction) is addressed and, more importantly,
mutation-tested: reverting the redaction on the `AllowAll` branch reproduces the
leak (`row TKT-1 leaked a hidden property`), so a regression cannot pass silently
- [x] All significant review-responses addressed or explicitly deferred with a
reason — RR-OXE47R and RR-SSPCCI addressed; RR-DFNBB3 deferred, see below
- [x] Self-reviewed the diff for unrelated changes — the diff is the two new
`internal` files plus their tests, a 26-line `runtime.go` wiring change, a
33-line `luareader.go` change, and the two Lua-scripting docs. No stray edits

**Review Responses:** RR-1W1G6K (critical, addressed), RR-OXE47R (significant,
addressed), RR-SSPCCI (significant, addressed), RR-DFNBB3 (significant,
deferred), RR-4DUSO1 (minor, addressed), RR-DSINTY (minor, addressed)

**Deferred finding — RR-DFNBB3 ("cap without a cursor is a dead end"):** correct,
and it changed the design rather than being waved through. The plan's "fail
loudly on truncation" mitigation was dropped as papering over a missing feature.
A real cursor is blocked on store-side paging (`store.GraphQueryer` has no
`Limit`/`Cursor`, verified against `develop` on 2026-07-29 — nothing merged,
nothing open), and both fakeable interims are worse than waiting: an inert cursor
makes the idiomatic paging loop infinite, an offset-backed one skips and
duplicates rows under concurrent writes. Stage 1 therefore makes `cursor` an
**error** rather than accepted-and-ignored, so no script can be written against a
cursor that never advances — which is what makes stage 2's real paging purely
additive. Interim gap is bounded and measured: a type over 2000 rows is
unreachable, and the largest in-tree type is 933. Tracked as DEC-IYHLNF stage 2.

## Acceptance Verification

- [x] Each acceptance criterion tested — see the evidence block in IMPL-L1F9EG
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

- Uniform read options across the read API — PASS
- Ceiling clamps and `limit` cannot exceed it — PASS (mutation 1: removing the
clamp let `{limit = 999999}` through)
- Unknown option keys rejected, not ignored — PASS (mutation 2: `{cursor = "abc"}`
was silently accepted)
- ACL pushdown preserves field-level redaction, including on `AllowAll` — PASS
(mutation 3, the #1188 class)
- Pushdown does not over-read the store — PASS (mutation 4: dropping the iterator
early-stop pulled 50 rows for a limit of 10; both row count and pull count assert,
so a slice-after-materialize implementation cannot pass)
- Iterator errors raise instead of returning zero rows — PASS (mutation 5,
restoring the pre-TKT-FVQ4 `break` truncated the list with no error)
- Scope-composition errors fail closed — PASS
(`TestListPushdown_ScopeErrorFailsClosed`)

End-to-end on the live graph (933 review-responses, 235 tickets):
`rela.list_entities` returned 933 and 235, exactly matching `ls | wc -l`, so
nothing truncates at real scale under the 2000 ceiling.
`scripts/dev-status.lua` runs unchanged.

## Documentation (enhancements only)

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated — `docs/lua-scripting.md` and
`docs-project/entities/guides/GUIDE-lua-scripting.md`
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-SB55PN

## Final Checks

- [x] Commit message explains the why, not just what — names the unbounded-read
and silent-error-swallow failure modes, not just "add opts"
- [x] No TODOs or FIXMEs left unaddressed — the cursor work is recorded in
RR-DFNBB3 and DEC-IYHLNF stage 2, not left as an inline TODO
- [x] Ready for another developer to use — the option surface is documented in
both Lua docs, and `limit = 0` / unknown keys produce a clear error rather than a
surprising default

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI
- [x] All CI checks pass — full matrix green; the `Rela Tickets` gate is resolved
by this done-transition
- [x] PR URL documented below

**PR:** https://github.com/sourcehaven-bv/rela/pull/1255
