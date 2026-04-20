---
id: REV-DP0AM
type: review-checklist
title: 'Review: Relocate .rela/ user-local state to user config directory (cross-platform)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`)
- [x] Lint clean (`just lint`)
- [x] Coverage maintained (`go test -cover` per-package; CI ratchet enforces)

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed (except the 4 explicitly deferred with reasons)
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:**

Addressed: RR-1L4QP, RR-9R1XN, RR-7YTW0, RR-GKBC3, RR-1PGHY, RR-0IW1B, RR-E6WT7,
RR-BCM7F, RR-G3J0P.

Deferred with rationale: RR-USAC5, RR-IIE5L, RR-R2RHK, RR-UL121, RR-SBJEV,
RR-SA5FB.

Also addresses earlier design-review findings: RR-R3MKP, RR-LDRW3, RR-H25H0,
RR-CDQ8O, RR-JEMLO, RR-HNN0C, RR-D4KC3, RR-DKYZN, RR-Z1AUY, RR-TUUZA, RR-VLKV5,
RR-98TZZ, RR-628G7, RR-242DF, RR-FS0SZ, RR-HQGU7, RR-5XEBB, RR-L3368, RR-QNCMP.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

- AC1 userstate package: PASS (fs_test.go, paths_test.go, validate_test.go)
- AC2 identity precedence: PASS (loader_test.go covers env / userstate / none)
- AC3 NewLocalState(svc): PASS (localstate_test.go)
- AC4 dataentry uses userstate: PASS (app_test.go round-trip)
- AC5 scheduler uses userstate: PASS (ws.State() returns userstate.Service)
- AC6 factory explicit constructor: PARTIAL — NewFSFactory present; struct
literal still allowed. Deferred to a cleanup follow-up (RR-IIE5L).
- AC7 `rela keys init` writes to us.Path("key"): PASS (keys.go)
- AC8 cross-platform path resolution: PASS (paths_test.go + CI matrix job)
- AC9 $RELA_USER_STATE_DIR validation: PASS (fs_test.go
TestNewFSWithRepoID_RejectsOverrideInsideProject)
- AC10 git-tracked check: PASS (repoid_test.go
TestResolveRepoID_RefusesTrackedByGit)
- AC11 0o600 / 0o700: PASS (fs_test.go TestNewForTest_FilePermissions)
- AC12 platform indexer opt-out: PASS (darwin writes marker,
windows sets attribute, linux no-op — verified by build tags)
- AC13 error-string audit: PASS (internal/cli/keys.go, internal/app/factory.go)
- AC14 no regression: PASS (just test + go test -race ./... green)

## Documentation (enhancements only)

- [x] User-facing documentation updated (`docs/encryption.md`)
- [x] CLAUDE.md updated with User-Local State section + path table
- [x] No docs-checklist needed: doc updates are inline with code changes;
the changes don't add new user-visible features, just document a moved file
layout.

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [ ] Run `/pr` command to create PR and monitor CI
- [ ] All CI checks pass
- [ ] PR URL documented below

**PR:** (to be added after creation)
