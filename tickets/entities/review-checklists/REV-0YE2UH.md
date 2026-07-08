---
id: REV-0YE2UH
type: review-checklist
title: 'Review: attachments: top-level key rejected by metamodel loader'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`go test ./...` + `just ci` green; CI matrix green except the ticket-status gate resolved by this transition)
- [x] Lint clean (`golangci-lint run` → 0 issues; CI Lint/Architecture/God-object pass)
- [x] Coverage maintained (`just coverage-check` → thresholds PASS)

## Code Review

- [x] Run `/code-review` (cranky-code-reviewer) — no critical issues; 1 significant, 1 minor, plus nits
- [x] ~~All critical review-responses addressed~~ (N/A: none raised)
- [x] All significant review-responses addressed (RR-G4A0Z1 — fixed)
- [x] Self-reviewed the diff for unrelated changes (only loader.go + 2 test files touched)

**Review Responses:**

- **RR-G4A0Z1** (significant) — *addressed*. HTTP tests originally used a struct-literal metamodel, so they didn't depend on the loader fix. Rewrote `newGlobalAttachmentsApp` to parse from a real YAML string via `metamodel.Parse`. Verified: reverting the fix now fails both HTTP tests at parse time with `unknown key "attachments"`.
- **RR-GHDQXC** (minor) — *deferred*. Sibling loaders (`dataentryconfig`, `acl`) share the same whitelist-drift class; reviewer flagged as out-of-scope follow-up. Filed **TKT-ELX09J**.
- Nits (`slices.Equal`, `sh`/`grep` non-Windows) — acknowledged, no change (POSIX-targeted, three-lines-beats-abstraction).

## Acceptance Verification

**Acceptance Status:**

- **AC1 — `rela validate` accepts the block:** PASS. Built `cmd/rela`, validated a project with `attachments: {allow, scan_cmd}` → `✓ valid`, exit 0.
- **AC2 — regression tests fail without the fix:** PASS. All 4 tests fail on revert (2 loader tests, 2 HTTP tests) with the exact `unknown key "attachments"` error; pass with fix.
- **AC3 — parity test guards the class:** PASS. `TestValidTopLevelKeysMatchStruct` catches any top-level struct field missing from the whitelist.
- **AC4 — global allow: enforced end-to-end:** PASS. `TestAttachmentUpload_GlobalAllowlistEnforced` (text 200, PNG 422 via HTTP).
- **AC5 — global scan_cmd: enforced end-to-end:** PASS. `TestAttachmentUpload_GlobalScanCmdRejects` (clean 200, INFECTED 422; scan proven to be the rejecting mechanism).

- [x] Each acceptance criterion tested
- [x] Test evidence documented in implementation checklist (IMPL-EKKU9N)

## Documentation (enhancements only)

- [x] ~~Docs-checklist~~ (N/A: bug fix, no user-facing behavior change — docs already document `attachments:`; this makes them work)
- [x] ~~User-facing documentation updated~~ (N/A)
- [x] ~~Docs-checklist marked as done~~ (N/A)

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI
- [x] All CI checks pass (full matrix green: cross-compile ×8, Frontend, Fuzz, God-object, Architecture, CodeQL, Analyze; the "Rela Tickets" gate is resolved by this done-transition; local `just ci` green)
- [x] PR URL documented below

**PR:** https://github.com/sourcehaven-bv/rela/pull/1097
