---
id: REV-WPVK8E
type: review-checklist
title: 'Review: lua: extend rela.bypass_acl''s admin handle with read methods (elevated reads, closure-scoped)'
status: done
---

## Automated Checks

- [x] `just lint` — 0 issues
- [x] `just arch-lint` — OK, no warnings. **No rule additions were needed**:
the `appbuild → principal` violation was fixed by moving the audit adapter to
`internal/audit` (which already owns `principal` and the record shape) rather
than widening the allow-list.
- [x] `just plimsoll` — passes. `Runtime` hit 125/120 methods; resolved by
making the read builders + `recordElevatedReads` **package functions**, not by
raising the directive.
- [x] `just lint-md` — 245 files, 0 issues
- [x] `just docs` — generated output regenerated and committed; the pre-push
gate (`git diff --exit-code docs/ README.md`) confirms sync
- [x] Race detector clean on all touched packages
- [x] Coverage floors satisfied (package 50% / total 65%)

## Code Review

- [x] `/code-review` run (cranky-code-reviewer) against commit `31813351`
- [x] All findings triaged and recorded as review-response entities
- [x] **All 3 findings addressed** (2 significant, 1 minor) — see
[[RR-FDV0GO]], [[RR-B89X8H]], [[RR-D7KXKV]]
- [x] No open critical or significant findings remain

### What the review confirmed

The reviewer independently verified the security core empirically rather than by
reading — worth recording because these were the properties I was least able to
prove to myself:

- **Coroutine escape is blocked.** A coroutine that yields the `admin` handle
out and calls it from the main thread raises "used outside its closure".
gopher-lua's `switchToParentThread` unwinds via panic, so the `defer` fires and
`live=false` lands before the handle escapes.
- **Nested-bypass revival is blocked** — a stale handle stays dead inside a
second closure.
- **`readUsage`'s single-goroutine assumption is sound** — no
goroutine/errgroup/WaitGroup anywhere in `lua`, `autocascade`, `script`, or
`automation`; coroutines share one OS thread.
- **Mid-iteration `RaiseError` is safe** — the iterator body does not resume,
deferred cleanup runs exactly once, the error reaches the script.
- **A Go panic (not a Lua raise) still audits** — `PCall` recovers, the
`defer` runs.
- **The audit row cannot be suppressed by failing, nor duplicated.**
- **The typed-nil test is genuinely load-bearing**, including its choice to
assert through `lua.WriteDeps` rather than the return value.

## Verification

- [x] Every fix mutation-verified: reverting each one fails its own
regression test (`StoreErrorIsNotMaskedAsMissing`,
`DeniedByArgValidationIsNotAudited`, `GetRelationsRejectsNonStringFilter`)
- [x] Both directions pinned, so no fix could swing too far —
`MissStillReturnsNil` guards the error-raising fix from breaking existence
checks; `GetRelationsAcceptsAbsentFilters` guards the type-check from rejecting
legitimately absent options
- [x] Full suite green **except**
`TestScriptReadSeam_PolicylessProjectStaysUnrestricted`, which fails identically
on a pristine `develop` worktree — pre-existing, fixed by PR **#1180/#1228**
(verified by checking out a clean worktree at `3974fecb`, not assumed)

## Notes

- `NewLuaScriptRunner` now has zero production callers (both wiring sites use
the elevated-reads constructor). Kept deliberately — "no read elevation" is a
real configuration, and the withheld-capability tests need it. Noted in its
godoc so the next reader is not puzzled.
- The identical non-string-filter shape in the **gated** `luaGetRelations`
was left as-is: tightening it is a breaking change on a binding scripts already
use, and the risk there is bounded by peer-gating. Follow-up territory, not a
drive-by change inside this ticket.

## PR

Branch `feat/tkt-acsbsa-elevated-reads`. PR to be opened once **#1228** lands
and clears the pre-existing `develop` failure.
