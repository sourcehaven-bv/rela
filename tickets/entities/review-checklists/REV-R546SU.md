---
id: REV-R546SU
type: review-checklist
title: 'Review: Faced types: attachment upload/download and entity export 404 on ID@face; file bytes shared across faces'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`): go test -race over dataentry, mcp, entitymanager, attachment, automation, cli, appbuild, archguard, cmd, metamodel, store, lua, autocascade all ok; e2e attachments-create, faces-backlog, faces-write, faces-reader: 28 passed, 5 skipped
- [x] Lint clean (`just lint`): golangci-lint 0 issues; arch-lint OK; plimsoll clean; postgres and sqlite builds pass
- [x] Comment lint gate clean (`just comment-lint`)
- [x] Coverage maintained (`just coverage-check`)

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent): cranky-code-reviewer and rela-security-reviewer
- [x] All critical review-responses addressed (none raised)
- [x] All significant review-responses addressed: RR-ZNYZ3W, RR-2AH46G, RR-392FWI, RR-VSYUX9
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-ZNYZ3W, RR-2AH46G, RR-392FWI, RR-VSYUX9, RR-QASM43,
RR-DMFKP1, RR-EQBMH0, RR-BRCRWK, RR-1GVHWO, RR-2C6GDH, RR-B6W9B6 (deferred to
TKT-7R0ABK), RR-B1CANH, RR-YL7XW2, RR-7852KJ, RR-7VUWD6, RR-WQKEKG, RR-U5ZJDR
(wont-fix).

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist (IMPL-385KRJ)

**Acceptance Status:**

- PASS: `_attachments` PUT/GET/DELETE on `ID@face` resolve the face (attachment_face_test.go, e2e BUG-CTUW2N describe).
- PASS: a face lists and serves only its own files; bytes are shared and dropped with the last reference (faced_test.go).
- PASS: an ordinary write changing a file value is a 422; automations naming a file property fail to load (attachments_test.go, fileprop_test.go).
- PASS: copies carry only the source face's references and take the entity lock (faced_test.go copy tests, race test).
- PASS: CLI and MCP take `ID@face` and refuse a bare id on a faced type naming the faces (cli and mcp tests).
- PASS: `_export` takes `ID@face` (dataentry export tests).

## Documentation (enhancements only)

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: bug fix)
- [x] User-facing documentation updated: content-states, acl-security, cli-reference, mcp-server and their docs-project mirrors
- [x] ~~Docs-checklist marked as done~~ (N/A: bug fix)

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI: done by the orchestrating agent after the ticket is done
