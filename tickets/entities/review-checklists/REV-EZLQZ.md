---
id: REV-EZLQZ
type: review-checklist
title: 'Review: Add cache API for Lua scripts (get/set + memoize, process-wide with per-script namespace)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`) — 30 cache-specific tests, full suite clean
- [x] Lint clean (`just lint`) — 0 issues
- [x] Coverage maintained (`just coverage-check`) — 72.7% total; internal/lua 85.2% against new 80% floor

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed (RR-4GQQ6)
- [x] All significant review-responses addressed (RR-LRLSJ, RR-392IH, RR-C819E)
- [x] Self-reviewed the diff for unrelated changes

**Review Responses (code review, post-implementation):**

Critical (1):

- `RR-4GQQ6` — Cyclic Lua tables crash process. **Addressed**: added
cycle detection via seen-set in `validateRepresentable`. End-to-end verified
cycle script now errors cleanly.

Significant (3):

- `RR-LRLSJ` — `luaCacheMemoize` validates while converting. **Addressed**:
split into two-pass validate-then-convert
- `RR-392IH` — `allowedOptionList` dead code. **Addressed**: replaced with
`sort.Strings` + `strings.Join`
- `RR-C819E` — Large TTL overflows silently. **Addressed**: added
`maxCacheTTLSeconds` guard rejecting values beyond int64 Duration capacity

Minor (4):

- `RR-65CJJ` — `SetNow` invariant. **Addressed**: doc comment expanded
- `RR-U5YYQ` — Concurrent test assertion too broad. **Deferred**
- `RR-LMIAE` — `TestCacheErrorMessagesDoNotLeakKey` dead code. **Addressed**
- `RR-BWHEV` — Process-wide overstated. **Addressed**

Nit (2):

- `RR-29B13` — NUL-in-path defensive assertion. **Deferred**: nit
- `RR-94W0R` — Rename `SetScriptPath`. **Deferred**: nit

No open critical or significant findings.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

AC 1–19: **PASS**. See IMPL-O1VDT for the full AC-to-test mapping.

## Documentation (enhancements only)

- [x] User-facing documentation updated
- [x] CLAUDE.md paragraph added

## Final Checks

- [x] Commit message will explain why
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] ~~Run `/pr` command to create PR and monitor CI~~ (N/A: user invokes `/pr` separately; all local checks pass)
- [x] ~~All CI checks pass~~ (N/A: will run on PR creation; verified locally)
- [x] ~~PR URL documented below~~ (N/A: added when PR is created)

**PR:** *(user to create via `/pr`)*
