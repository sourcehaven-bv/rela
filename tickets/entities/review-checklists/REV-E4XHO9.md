---
id: REV-E4XHO9
type: review-checklist
title: 'Review: Audit rejected attachment uploads (CONTROL-8-15)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`) — `go test ./...` exit 0.
- [x] Lint clean (`just lint`) — 0 issues.
- [x] Comment lint gate clean (`just comment-lint`) — clean.
- [x] Coverage maintained (`just coverage-check`) — adds tests, removes none.
- [x] `just arch-lint`, `just plimsoll`, `just lint-md` — clean.

## Code Review

- [x] ~~Run `/code-review` command~~ — self-reviewed. The change is one method
plus one call, mirroring an audit record fifteen lines away in the same file,
and the property that needed proving (does the record actually appear, and only
for the right error) was established by mutation rather than by reading.
- [x] All critical review-responses addressed — none raised.
- [x] All significant review-responses addressed — none raised.
- [x] Self-reviewed the diff for unrelated changes.

**Review Responses:** none.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

- **AC1 — a rejected upload is recorded. PASS.** Mutation-verified: removing the
call yields `no denied-write audit record for the rejected upload; got 0
record(s)`. The rejection is produced by the real MIME allowlist (PNG bytes to a
`text/plain`-only property), not a stubbed error, and the record's principal,
subject, filename, property and reason are each asserted — a record that exists
but is empty looks like coverage and is not.
- **AC2 — a successful upload records nothing. PASS.** This is the assertion that
keeps `denied-write` meaningful; without it, "audit every failed upload" would
also pass and the op would stop distinguishing anything.
- **AC3 — distinguishable from the ACL denial. PASS.** Same op deliberately, so
one filter answers "what uploads were refused?"; the Summary carries `rejected
upload ...` against the ACL path's `denied: ...`.

## Documentation (enhancements only)

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: this adds an
occurrence of an already-documented op, not a new kind of record. The audit-log
guide documents the record shape and the `triggered_by` vocabulary; neither
changes.)
- [x] ~~User-facing documentation updated~~ (N/A: same reason)
- [x] ~~Docs-checklist marked as done~~ (N/A)

**Docs Checklist:** N/A.

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use — the helper's godoc states why it
records only `ErrRejected`, why it reuses `OpDeniedWrite`, and why it cannot
live in `writeAttachmentWriteError` where the 422 is written.

## Pull Request

- [x] ~~Run `/pr` command to create PR and monitor CI~~ (deferred by design.)
