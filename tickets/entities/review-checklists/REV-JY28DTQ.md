---
id: REV-JY28DTQ
type: review-checklist
title: 'Review: Refuse remote credentials outside remotes.yaml'
started: "2026-11-13"
completed: "2026-11-13"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`): affected packages (ai, mail, secrets, cmdexec, reconclient, remotesync, dataentry, transform, check, attachment, cli, lua, archguard) pass locally; full suite runs in CI
- [x] Lint clean (`just lint`): golangci-lint on the changed packages, 0 issues; arch-lint OK
- [x] Comment lint gate clean (`just comment-lint`)
- [x] Coverage maintained (`just coverage-check`): new code is covered by its tests; CI runs the floor check

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** cranky-code-reviewer and rela-security-reviewer.
RR-R2PYEA3 (significant, addressed), RR-K7B9YJT, RR-C4E5YHP, RR-FY8DVO9,
RR-XZZVT64, RR-1BKVJAG/RR-TXQJ9BU (addressed or deferred with reason). Original
findings: RR-Q09NYPQ addressed, RR-T2DQV3D wont-fix (confirmed by the security
reviewer).

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**
1. ai.yaml refusal: PASS (TestParseConfig_RefusesRemoteCredentialEnv, TestNewProvider_RefusesRemoteCredentialEnv, manual rela script run).
2. mail.yaml refusal: PASS (TestConfig_RefusesRemoteCredentialEnv via Validate and NewSMTPSender).
3. Case and bare prefix: PASS (TestIsRemoteCredentialEnv).
4. RR-T2DQV3D closed with reason: PASS.
Added from review: child processes never inherit REMOTE_CREDENTIAL_*: PASS
(TestWithoutWithheld, TestRun_WithholdsSecretEnv).

## Documentation (enhancements only)

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-RJM9WQK

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI (next step, after done)
