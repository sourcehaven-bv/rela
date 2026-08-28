---
id: REV-5JNW5K
type: review-checklist
title: 'Review: Asserted roles are inert on the production JWT gate — claims dropped at requireVerifiedJWT'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated checks

- [x] `just test` (full suite) — pass, no failures
- [x] `go test -race ./internal/dataentry ./internal/jwtauth` — clean
- [x] `just lint` — 0 issues
- [x] `just arch-lint` — OK; jwtauth stays a leaf, dataentry does not import it
- [x] `just coverage-check` — PASS (76.3%)

## Code review

- [x] `cranky-code-reviewer` run against the branch diff
- [x] All findings addressed

**Findings: 2 (1 minor, 1 nit). Both addressed. Zero critical/significant.**

| ID | Severity | Finding | Status |
|---|---|---|---|
| RR-LP9XAU | minor | Gate role-bounds/sanitization not exercised end-to-end through the gate | addressed |
| RR-E9N3WY | nit | `verifiedPrincipal` doc overstated "only constructor" (UnmarshalJSON too) | addressed |

The reviewer verified the load-bearing properties by tracing and running code,
not by inspection:

- **Fail-closed unchanged.** Both `VerifySubject` and `VerifyAssertion` route
their parse error through the same `classify()` (verifier.go:138 / :189,
byte-identical), so ErrKeysUnavailable-vs-ErrInvalid and the Error-vs-Info
logging split are reached identically. Pinned by the existing
`TestVerifyAssertion_MatchesVerifySubject`.
- **No empty-subject bypass.** `VerifyAssertion` returns `ErrInvalid` for an
empty subject (denies before `verifiedPrincipal`), and `verifiedPrincipal`
guards `Subject==""` → `ok=false` anyway. No path yields `ok=true` on an empty
subject.
- **Adapter preserves error classification.** The reviewer ran a scratch program
confirming the double-`%w` wrap keeps `errors.Is(err, ErrKeysUnavailable)` true
across the adapter — so the gate's KeysUnavailable predicate still fires.
- **The end-to-end test is not a tautology.** The policy grants read ONLY via the
asserted claim; the principal has no assignment and no membership edge, so 200
can only come from the claim flowing through. The negative subtest rules out a
default-open grant.

Both findings were addressed with a fault-injected test / a doc precision fix.
The new sanitization test fails correctly when `verifiedPrincipal`'s role loop
is replaced with a raw passthrough.

## Acceptance verification

AC1-AC5 verified (see IMPL-FY4LWA table). AC5 (regression guard) confirmed by
reverting the fix and observing the tests fail with diagnostics naming the
dropped claims — the systemic guard why5 identified as missing now exists and
demonstrably catches the exact bug.

## Docs

- [x] ~~Project docs~~ (N/A: code-only change; the shipped docs already describe
asserted roles working — this makes the code match them, no drift)
- [x] Godoc on the new `verifiedPrincipal` helper, the widened `JWTGateConfig`,
and the `assertionVerifierAdapter`
