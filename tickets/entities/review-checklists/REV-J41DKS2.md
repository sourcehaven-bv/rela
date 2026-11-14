---
id: REV-J41DKS2
type: review-checklist
title: 'Review: rela test: stub and record outgoing HTTP calls'
started: "2026-11-14"
completed: "2026-11-14"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`)
- [x] Lint clean (`just lint`)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] Coverage maintained (`just coverage-check`)

`go test ./...` passes except `cmd/rela-desktop`
`TestChromeStyle_TargetsShippedClasses`, which needs the built SPA ("no
stylesheets in the embedded SPA") and is untouched by this change. `just lint`,
`just arch-lint`, `just plimsoll` and `just comment-lint` are clean.

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-RW4BSM3 (critical), RR-UJOZ83F, RR-ABJHIM6
(significant), RR-6MG8WAC (security, minor), RR-UOKCUVQ, RR-WXU5N4B, RR-ZEZMQ9A,
RR-V4ZEEBL, RR-SYRCFI0, RR-UZRJHVZ (minor, addressed), RR-3D8XMM3 (minor,
wont-fix), RR-O4NP5QD (nit).

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**
1. PASS: `TestHTTPStub_Answers` (exact URL, host case, prefix, pattern, headers, JSON and form subsets).
2. PASS: `TestHTTPStub_Answers` (status, headers, body, json, fixture file, error kinds, sequences with an error).
3. PASS: `TestHTTPStub_Failures` (unmatched lists stubs, no stubs, swallowed, spent, earlier assertion wins, location).
4. PASS: `TestHTTPStub_Answers` (`stub:calls()`, `stub:count()`, `t.http.calls()`).
5. PASS: "no real secrets", `TestHTTPStub_WebhookSignedWithFakeSecret`, and no failure message holds the real secret.
6. PASS: `TestHTTPStub_FlowJobRetry` (503 then 201 at the 30 s backoff, 4xx permanent, webhook network error then 204).
7. PASS: "outside", "cache dir", "cache dir upper case", `TestHTTPStub_FixtureSymlinkRefused`.

## Documentation (enhancements only)

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-LFICFXI. The guide itself is TKT-E2IMTMG.

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI
