---
id: IMPL-EAK7M
type: implementation-checklist
title: 'Implementation: Close the =~ ReDoS hole: require trusted literal regex patterns (issue #1139)'
status: done
---

<!-- @managed: claude-workflow v1 -->

> **Approach changed mid-implementation.** The value-length cap was built first
> (as issue #1139 proposed), then disproved by code review and abandoned — see
> RR-HPQV2. This records what actually shipped.

## Development

- [x] Unit tests written for new code
- [x] Integration tests written (test full flow, not just units) — tests drive the public `parse()` / `parse().eval()` path, not `compareRegex` in isolation
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] Error handling in place (errors surfaced, not swallowed)

Changes (`frontend/src/utils/conditions.ts`):

- **The parser rejects any non-literal `=~` pattern** — throws `ConditionError`
at parse. This is the fix: it removes the untrusted-pattern path rather than
mitigating it. Static, so it fails loud like every other config bug.
- **`compareRegex` simplified.** The pattern is now guaranteed to be a
parse-validated literal, so the dynamic-pattern length cap and the compile-time
`try/catch` became dead code and were **deleted**. Net effect: the function
shrank.
- **`MAX_MATCH_VALUE_LENGTH = 10_000`** kept, but documented as *hygiene*
(bounding a linear scan of an untrusted value — e.g. a pasted megabyte on the
render thread), explicitly **not** a ReDoS boundary.
- **Threat model documented** in the module doc; three pre-existing docstrings
corrected (they claimed length caps bound ReDoS — now disproven).
- **Parse-cache comment** notes the cache is unbounded *by design*, safe only
because sources are operator config (second-review finding).

`frontend/src/utils/conditions.test.ts`: the ReDoS block was rewritten. Tests
now pin the parse-time rejection of data-sourced patterns (the actual control)
plus the value cap's exact boundary. All wall-clock timing assertions removed.

## Test Quality

- [x] Using fixture builders or factories for test data — reuses the file's existing `parse` / `evalWith` helpers and the established `vi.spyOn(console,'warn')` pattern
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] ~~Interpolated values constructed from objects~~ (N/A: assertions match stable warn substrings / thrown-error regexes)
- [x] ~~Property comparisons use original object~~ (N/A: boolean and throw assertions only)

Tests earn their keep — verified, not assumed:
- 3 of 4 new tests **fail against the unmodified code** (checked by stashing the
implementation).
- The boundary test is **mutation-tested**: cap 10_000→5_000 ⇒ fails (1 failed /
40 passed); restored ⇒ 41/41. This fixed RR-G26WN, where the original boundary
test passed on the *unmodified* code — a false coverage claim.

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified (restated — the planning ACs assumed the disproven approach; see REV-MJ27P)
- [x] Edge cases manually verified

**Verification Evidence:**

Suite: **79 files / 1312 tests passed** (`conditions.test.ts` 41/41). Typecheck
(`vue-tsc --noEmit`) clean. Lint 0 errors, **0 warnings in the two changed
files**. Prettier clean on both.

Empirical measurements against the real JS engine (this is what killed the
original approach):

| Case | Result |
|---|---|
| `(a+)+$` vs 41-char non-matching value | **hangs >60s** — pattern 6 chars (under the 200 cap), value 41 chars (240× under a 10k cap) |
| `(a+)+$` vs 27-char non-matching value | **~9.7s** |
| `(a+)+$` vs 100k all-`a`s | **0.11ms** — it *matches*; no backtracking (why the first regression test was hollow) |
| `parse('form.v =~ form.pat')` | **throws in 2ms** — the attack is now unreachable |

- **AC1′** untrusted pattern cannot reach the engine: **PASS** — rejected at
parse; reviewer independently probed 6 bypass variants (paren-wrapped ref,
reversed operands, `not`, call, non-string literals), all rejected.
- **AC2′** no regression on literal patterns: **PASS** — 1312/1312 green.
- **AC3′** eval never throws; over-cap value fail-safe: **PASS**.
- **AC4′** cap honestly described and actually pinned: **PASS** (mutation test).

## Quality

- [x] Code follows project patterns — the parse-time throw matches the engine's documented contract (parse throws on static config bugs; eval never throws). `=~` now has *zero* eval-time pattern failure modes, which tightens that contract rather than bending it.
- [x] Checked for DRY opportunities — the change **removes** code (dead cap + dead try/catch) rather than adding abstraction.
- [x] No security issues introduced — closes the untrusted-pattern hole; warn messages leak no value content, only the cap.
- [x] No silent failures — rejection is warned and returned; the parse rejection throws loudly.
- [x] No debug code left behind — scratchpad/adversarial probes deleted, never committed.
