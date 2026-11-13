---
id: IMPL-TSYD2JI
type: implementation-checklist
title: 'Implementation: Refuse remote credentials outside remotes.yaml'
started: "2026-11-13"
completed: "2026-11-13"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] Integration tests written (test full flow, not just units)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] Error handling in place (errors surfaced, not swallowed)

## Test Quality

- [x] ~~Using fixture builders or factories for test data~~ (N/A: config literals)
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] ~~Interpolated values constructed from objects~~ (N/A: no interpolation)
- [x] ~~Property comparisons use original object~~ (N/A: error-text assertions)

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**
- Built rela, temp project with .rela/ai.yaml `api_key_env: REMOTE_CREDENTIAL_UP`, `rela script t.lua` calling ai.chat: "invalid ai.yaml: api_key_env may not name "REMOTE_CREDENTIAL_UP": a remote credential is only for remotes.yaml and rela reconcile". No request sent.
- mail has no CLI entry; TestConfig_RefusesRemoteCredentialEnv checks Validate and NewSMTPSender.
- TestRun_WithholdsSecretEnv runs `env` as a real child with REMOTE_CREDENTIAL_UP set; the value is absent.
- Mutation check: disabling each Validate refusal fails its tests.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities (one prefix constant in the secrets leaf; httpremote alias dropped)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
