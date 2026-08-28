---
id: IMPL-TISRMZ
type: implementation-checklist
title: 'Implementation: Clear all doclink findings and promote the rule to a blocking CI gate'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] ~~Unit tests written for new code~~ (N/A in this repo: every Go change
is inside a comment. The two upstream changes ship with tests — `TestGuidance`,
`TestFiredRules`, `TestStripBackticked`.)
- [x] ~~Integration tests written~~ (N/A: the integration is the CI gate,
verified by running it including a negative case)
- [x] Happy path implemented — `doclink` reports zero; the gate includes it
- [x] Edge cases from planning handled — the backticked-markdown false
positive is fixed upstream with a test
- [x] Error handling in place — the blocking path prints guidance rather than
just exiting non-zero

## Test Quality

- [x] ~~Using fixture builders or factories for test data~~ (N/A: no tests
added in this repo)
- [x] ~~No hardcoded values in assertions when object is in scope~~ (N/A)
- [x] ~~Only specifying values that matter for the test~~ (N/A)
- [x] ~~Interpolated values constructed from objects, not hardcoded~~ (N/A)
- [x] ~~Property comparisons use original object, not hardcoded strings~~ (N/A)

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

1. **Zero findings** — `commentlint -rules doclink ./internal ./cmd` →
"no unresolvable doc links across 10351 comments" (was 64).
2. **Gate includes it and passes** — `just comment-lint` (now
`-rules commented-code,doclink`) → EXIT=0.
3. **The gate actually blocks** — reverted one fix
(`[Set.EnforceUpdate]` → `[Set.Enforce]`), re-ran: EXIT=1 with the guidance
printed, naming `doclink` in the copy-pasteable directive. Restored after.
Without this the promotion would be cosmetic.
4. **No behaviour change** — all 63 added Go lines are comments:

   ```
   $ git diff -U0 -- '*.go' | grep '^+' | grep -v '^+++' | grep -vE '^\+\s*//'
   (no output)
   ```

5. **Nothing regressed** — `golangci-lint run ./internal/...` → 0 issues;
`go test` over every touched package → no failures; `just lint-md` → 0 issues
across 252 files.

## Quality

- [x] Code follows project patterns — the gate mirrors the existing
`commented-code` step and the plimsoll job's pinned-version convention
- [x] Checked for DRY opportunities — the guidance message lives in one
function (`failWithGuidance`) routed to from all five exit sites upstream,
rather than duplicated per rule
- [x] No security issues introduced — every edit is inside a comment. Several
touch security-relevant packages (`acl`, `visibility`, `jwtauth`), so the diff
was checked line-by-line to confirm nothing outside a comment moved. One comment
is *more* accurate now: `[Declarative.ReadQuery]` was pointing at a method that
does not exist; it is `Request.ReadQuery`.
- [x] No silent failures — the gate fails loudly and explains what to do
- [x] No debug code left behind

**Judgement calls worth recording:**

- **Cleared to zero rather than grandfathering.** The plimsoll pattern (pin
the count, ratchet down) does not fit: the growth was existing breakage
*propagating*, so a frozen ceiling still lets each broken reference spread.
- **Did not suppress any of the 64.** Suppressing findings this mechanical
would make the escape hatch the default path — exactly the reflex the guidance
message is written to discourage.
- **`grep` was the wrong oracle.** An earlier audit graded 7 of 8 sampled
findings as false positives because the symbols "exist". They do — as methods —
and Go still will not link a bare `[Method]`. Every shape was re-verified
against `go doc` on a minimal package before being fixed.
