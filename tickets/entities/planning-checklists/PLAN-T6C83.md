---
id: PLAN-T6C83
type: planning-checklist
title: 'Planning: ReDoS mitigation in conditions.ts must cap matched-value length (not only pattern length)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

The `=~` operator in `frontend/src/utils/conditions.ts` compiles a regex and
runs `re.test(String(value))` on the main thread. JS's backtracking RegExp
engine has no match timeout. RR-IROUO added `MAX_REGEX_LENGTH=200` to cap the
**pattern** length — but catastrophic backtracking (`(a+)+$` and friends) is
driven by the length of the **input**, not the pattern. A 5-char pattern against
a 50k-char string of `a`s hangs the thread; the pattern cap does nothing for it.
So the original critical finding is only partially mitigated, and the regression
test only exercises the oversized-pattern path.

**Scope:**

IN scope:
- Cap the length of the matched value (`String(value)`) in `compareRegex`
(`conditions.ts:620`) — reject with `console.warn` + `false` when over the cap.
Fail-safe, never throws (mirrors the existing pattern-length cap for the dynamic
path).
- Introduce a `MAX_MATCH_VALUE_LENGTH` constant with a doc comment referencing
the ReDoS rationale.
- Add a regression test reproducing short-pattern / long-value catastrophic
backtracking and asserting it returns quickly.

OUT of scope:
- Wiring the engine to any consumer (that is TKT-CHLAJ).
- Moving regex evaluation off-thread (Web Worker) or swapping to a
linear-time engine (RE2 / re2-wasm) — heavier, not warranted while dormant.
- Any change to `validateRegexLiteral` (parse-time pattern check) — the value
is not knowable at parse time, so the value cap belongs only at eval time in
`compareRegex`.
- Go-side `internal/predicate` (separate engine, not affected).

**Acceptance Criteria:**

1. `compareRegex` rejects a value longer than `MAX_MATCH_VALUE_LENGTH` with a
`console.warn` and returns `false` **before** running `re.test`. *Test:* dynamic
eval `form.v =~ form.pat` with a short pathological pattern (`(a+)+$`) and a
long value (e.g. 100k `a`s) returns `false` in < 100ms.
2. A normal-length value still matches / fails to match as before (no
behavioural regression). *Test:* existing `=~` match/non-match cases still pass;
a value at exactly the cap length still evaluates.
3. The value cap is fail-safe (warn + false), never throws — consistent with
the dynamic-pattern cap. *Test:* over-cap value produces `false` + a warn call,
no exception.

## Research

- [x] ~~run `/research`~~ (N/A: small, well-scoped hardening; approach dictated by the issue)
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A — single-function fix, approach specified in rela#1139.

**Existing Solutions:**

- The existing pattern-length cap in the same file (`conditions.ts:608`,
`:627`) is the direct precedent: same coarse-ceiling strategy, same fail-safe
(warn + false) treatment on the dynamic path. The value cap is a symmetric
extension of it.
- Prior art: RR-IROUO (the original critical ReDoS finding on TKT-BL7XZ) and
its resolution establish the "coarse length ceiling, fail-safe, defence in depth
(patterns are trusted config)" posture this ticket continues.
- Linear-time engines (RE2/re2-wasm) were considered as an alternative and
rejected for now (see Approach) — disproportionate for a dormant utility.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

1. Add `const MAX_MATCH_VALUE_LENGTH = 10_000` near `MAX_REGEX_LENGTH`
(`conditions.ts:81`) with a doc comment: bounds ReDoS exposure on the *input*
side; the input length is what drives catastrophic backtracking, so this is the
primary of the two caps. 10k is comfortably above any real form-field /
entity-property value yet far below a hang-inducing size for even the worst
polynomial patterns.
2. In `compareRegex`, compute `const target = String(value)` **once**, then
before `re.test`, guard:
   ```ts
   if (target.length > MAX_MATCH_VALUE_LENGTH) {
     console.warn(`[conditions] =~ value too long (>${MAX_MATCH_VALUE_LENGTH} chars); rejected`)
     return false
   }
   ```
Placed after the pattern-length / compile guards so we don't stringify or test
when the pattern is already rejected. Fail-safe: warn + `false`, never throw —
identical contract to the pattern cap on this dynamic path.
3. No change to `validateRegexLiteral` — the value is unknowable at parse
time, so the cap has no analogue there.

**Alternatives considered:**

- **Off-thread execution (Web Worker) / linear-time engine (RE2-wasm):**
eliminates the hang class entirely but adds a dependency / async boundary and
bundle weight — disproportionate for a dormant utility with trusted-ish config
patterns. Rejected for now; the length cap is the documented coarse mitigation,
consistent with RR-IROUO.
- **Cap the value at `validateRegexLiteral`:** impossible — value not known at
parse time. Rejected.
- **Truncate the value instead of rejecting:** would silently change match
semantics (a match could appear/disappear based on truncation). Rejecting
fail-safe (false) is more predictable and matches the pattern-cap behaviour.

**Files to modify:**

- `frontend/src/utils/conditions.ts` — add constant + guard in `compareRegex`;
update the doc comment on `compareRegex` / `MAX_REGEX_LENGTH` block.
- `frontend/src/utils/conditions.test.ts` — add regression test(s) in the
"ReDoS guard on =~ (RR-IROUO)" describe block.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

- **Matched value** (`value`): comes from client-side bindings —
`form.<field>` / `entity.<field>` / `current_user.<field>`. Once TKT-CHLAJ wires
consumers, `form.*` is directly attacker-controllable (a user typing in a form
field). Validation: bounded length (`MAX_MATCH_VALUE_LENGTH`); over-cap → reject
fail-safe (false + warn). This is the gap this ticket closes.
- **Pattern** (`pattern`): config (trusted-ish) for literals; for dynamic
`form.v =~ form.pat` it can also be user data. Already length-capped
(`MAX_REGEX_LENGTH`). Unchanged.

**Security-Sensitive Operation:** unbounded backtracking regex on the render
thread = client-side DoS (tab hang). Bounding input length is a defence-in-
depth ceiling. No sensitive data in the warn message (it states only the cap,
not the value).

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios (map to acceptance criteria):**

- AC1 → new test: `form.v =~ form.pat` with `pat = '(a+)+$'` and
`v = 'a'.repeat(100_000)` returns `false` and completes in < 100ms
(`performance.now()` delta), and `console.warn` was called with a "value too
long" message. This is the scenario the original finding described and that the
existing suite never exercised.
- AC2 → existing `=~` match/non-match tests remain green; add a boundary case:
a value at exactly `MAX_MATCH_VALUE_LENGTH` still evaluates (not rejected).
- AC3 → assert the over-cap path returns `false` (not a throw) — covered by
the AC1 test's non-throwing expectation + warn assertion.

**Edge Cases:**

- Value exactly at cap → allowed. Value cap+1 → rejected.
- `NIL` value → already short-circuited to `false` before the cap
(`compareRegex:621`); unaffected.
- Long value with a *benign* pattern (e.g. `^foo`) that is over the cap → also
rejected (false + warn). Documented as acceptable: >10k-char values against `=~`
are not a real use case, and we cannot cheaply know a pattern is
backtracking-safe. Called out in the test as intended behaviour.

**Negative Tests:** over-cap value must yield `false` + warn, never throw and
never call `re.test` (verified via timing bound; a real `test` run of the
pathological pair would exceed the 100ms bound by orders of magnitude).

**Integration approach:** the engine is a pure utility with a public
`parse().eval()` surface; the regression test drives the full parse→eval path
(not `compareRegex` in isolation), which is the realistic integration for a
dormant engine. No server round-trip involved.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

- *Cap too low rejects legitimate values.* Mitigation: 10k chars is far above
any realistic form-field/property value matched by `=~`; picked with margin.
Easy to raise later if a real case appears (single constant).
- *Behavioural change for existing dynamic `=~` users.* Mitigation: engine is
dormant (no consumer), so no live behaviour changes; landing before TKT-CHLAJ
means the cap is in place before the first consumer.

**Effort:** s (single function + constant + tests).

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**

- [x] N/A — internal client utility, dormant, no user-facing surface yet. The
behaviour is documented in-code (doc comment on the new constant and the
`compareRegex` guard). No `docs/*` page describes `=~` value limits today, and
adding one before TKT-CHLAJ exposes the engine would be premature.

## Design Review

- [x] ~~Run `/design-review`~~ (N/A: single-function security hardening with the approach dictated by the issue; symmetric to the already-reviewed pattern cap)
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** None — no design review run (see above).
