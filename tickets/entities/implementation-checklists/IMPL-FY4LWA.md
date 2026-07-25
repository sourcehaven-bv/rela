---
id: IMPL-FY4LWA
type: implementation-checklist
title: 'Implementation: Asserted roles are inert on the production JWT gate — claims dropped at requireVerifiedJWT'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Feature implemented
- [x] Unit tests written for new code
- [x] Integration tests written (full flow, not just units)
- [x] All edge cases handled

**What changed (5 files, code-only — no doc change):**

1. `router.go` — extracted `verifiedPrincipal(AssertedIdentity) (Principal, bool)`,
the single place the assertion claims become a Principal. Both the gate and the
deprecated `JWTPrincipalResolver` call it, so they cannot drift on subject
sanitize / role filter — the drift that caused this bug. Removed the now-dead
`subjectVerifier` interface; updated `assertionVerifier`'s doc to name both
consumers.
2. `jwtgate.go` — the core fix. `JWTGateConfig.Verifier` is now
`assertionVerifier`; `requireVerifiedJWT` calls `VerifyAssertion` and stamps via
`verifiedPrincipal`, carrying org/roles. Fail-closed logging, the
unusable-subject denial, and `noteRecovery` are unchanged.
3. `cmd/rela-server/main.go` — added `assertionVerifierAdapter` (mirrors the
existing `webhookVerifierAdapter`) so `*jwtauth.Verifier` reaches the gate as
`dataentry.AssertedIdentity`. dataentry still does not import jwtauth (arch-lint
confirms).
4. `jwtgate_test.go` — stub `gateVerifier` now implements `VerifyAssertion` with
optional org/roles; every existing test passes unmodified.
5. `webhook.go` — stale comment referencing `subjectVerifier` updated.

## Manual verification

- [x] Each acceptance criterion verified
- [x] Verification evidence documented

| AC | Test | Result |
|---|---|---|
| AC1 | `TestJWTGate_AssertedRolesReachACL/asserted_role_grants_read_through_the_real_gate` — drives a signed assertion with `roles:[admin]` through the real `NewRouter`; the mapped viewer role grants read (200). Negative half asserts no-claim → not-200. | PASS |
| AC2 | `TestRequireVerifiedJWT_StampsAssertedClaims` asserts OrgID/OrgSlug on the stamped Principal. | PASS |
| AC3 | Existing `TestRequireVerifiedJWT` (JWKS-outage / keys-unavailable rows) pass unmodified; `VerifyAssertion` uses `classify()`, so fail-closed is unchanged. | PASS |
| AC4 | The AC1 test uses subject `usr_nobody` with NO assignments entry and NO membership edge — a principal that matches no user entity — and it still gets read via the asserted role. AC10 parity, end-to-end. | PASS |
| AC5 | **Fault-injected.** Reverting the gate stamp to the old subject-only literal fails both new tests with *"the gate dropped the org claim"*, *"the gate dropped the asserted roles"*, and the end-to-end read 404s. Restored → pass. | PASS |

The AC1/AC4 end-to-end test is the systemic guard why5 identified as missing: it
pins verify → stamp → resolve → ACL, so a future rework of the gate or the
resolver cannot silently sever claims again. Confirmed it catches the exact
regression by reverting the fix.

## Quality

- [x] Code follows project patterns (adapter mirrors `webhookVerifierAdapter`;
shared helper mirrors the consumer-side-interface idiom)
- [x] No silent failures

- `go build ./...` — clean
- `just test` (full suite) — pass, no failures
- `go test -race ./internal/dataentry ./internal/jwtauth` — clean
- `just lint` — 0 issues (removed the orphaned `subjectVerifier`)
- `just arch-lint` — OK; **jwtauth stays a leaf, dataentry does not import it**
- `just coverage-check` — PASS (76.3%)
- No docs touched → no `generate-docs` drift (the shipped docs already describe
asserted roles working; this makes the code match them).
