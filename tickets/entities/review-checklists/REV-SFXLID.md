---
id: REV-SFXLID
type: review-checklist
title: 'Review: CLI, MCP, Lua, automation and importer writes read the zero-face row'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`)
- [x] Lint clean (`just lint`)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] Coverage maintained (`just coverage-check`)

Also clean: arch-lint, plimsoll, sqlite/postgres/memorybackend builds, sqlite
tests of the touched packages, and the postgres suites (pgstore, entitymanager
face/cascade/D4/concurrency, dataentry webhook) against a local database.

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-F7AZKE, RR-YEHY3O, RR-TQFFLF, RR-MO5KSY, RR-6RH3W6,
RR-VJSEBZ, RR-X9ESXX, RR-0VLT7X, RR-2466U1, RR-UM7R2X, RR-V8M81O

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

- CLI delete and unlink on faced types: PASS (`internal/cli/delete_face_test.go`, manual run on a temp project).
- MCP delete_entity and delete_relation per face: PASS (`internal/mcp/delete_face_test.go`).
- Lua delete_entity and delete_relation per face: PASS (`internal/lua/face_write_test.go`).
- Automation create_relation and cascade-host delete keep faces and tails: PASS (`internal/entitymanager/writepaths_face_test.go`).
- Importer faced rows and tails: PASS (`internal/importer/importer_face_test.go`).
- D4 relation ACL: PASS (`internal/entitymanager/relation_d4_test.go`, acl tests).
- pg DeleteEntityState family lock: PASS (`internal/store/pgstore/facedeleterace_test.go`).
- Zero-face allowlist shrank by 13 reads in 6 files: PASS (archguard test).

## Documentation (enhancements only)

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: bug fix)
- [x] ~~User-facing documentation updated~~ (N/A: bug fix; `docs/lua-scripting.md` face table corrected anyway)
- [x] ~~Docs-checklist marked as done~~ (N/A: bug fix)

**Docs Checklist:** N/A

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] ~~Run `/pr` command to create PR and monitor CI~~ (N/A: the PR is opened with gh after the bug is done, per TKT-UFV01M)
