---
id: IMPL-42Y4CW
type: implementation-checklist
title: 'Implementation: Access PR 1: golden wire-parity corpus, test ACL helpers, verifiedIdentity rename'
started: "2026-10-12"
completed: "2026-10-12"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code (recorders_test.go: SoftDelete/MentionSources forwarding and a Tx race test for Counting, Breadth, BodyWatch)
- [x] Integration tests written (TestGoldenParity: 491 cases through the real router on an app built by NewApp)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] Error handling in place (errors surfaced, not swallowed)

## Test Quality

- [x] Using fixture builders or factories for test data (golden_fixture_test.go builds the fixture with fixed ids)
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] ~~Interpolated values constructed from objects~~ (N/A: assertions are recorded files)
- [x] ~~Property comparisons use original object~~ (N/A: same)

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**
1. `go test -count=1 ./internal/dataentry/ ./internal/store/... ./internal/acl/`: ok.
2. `go test -count=20 -run TestGoldenParity ./internal/dataentry/`: ok (363 s); `go test -race -run TestGoldenParity`: ok.
3. Removing the `when:` on the viewer's `secret` grant fails 5 cases, each with a short diff (`+ "secret": "bravo-secret"`); reverted.
4. Corpus: Declarative with header identity (70 endpoints x 6 identities), Declarative with JWT gate (20), NopACL, ReadOnlyACL, nil (17 each). Route guard over the router walk; exemptions `git`, `_git` (no repository in the fixture), `_openapi.json` (static).
5. Codemod: 365 sites in 104 files (285 setTestACL, 80 setTestResolver); `rebindApp`'s `app.acl = svc.ACL()` and struct literals stay raw by design.
6. `jwtWired` absent from *.go. golangci-lint 0 issues; comment-lint, arch-lint, plimsoll clean.
7. Deviations: three random values normalized (comment ids, attachment path segment, script-error correlation ids); cases run serially (global webhook header map); `/_settings` body lines sorted because of map order, filed as BUG-E0CZZL. `*.golden -text` in .gitattributes keeps feed CRLF bytes intact.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
