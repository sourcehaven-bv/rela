---
id: REV-8BYDPG
type: review-checklist
title: 'Review: Audit rela acl who-can queries (CONTROL-8-15)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`) — `go test ./...` exit 0.
- [x] Lint clean (`just lint`) — 0 issues (caught a `perfsprint`
`Sprintf`-to-concat).
- [x] Comment lint gate clean (`just comment-lint`) — clean.
- [x] Coverage maintained (`just coverage-check`) — adds tests, removes none.
- [x] `just arch-lint`, `just plimsoll` — clean. `just lint-md` clean after two
fixes in the new docs table.

## Code Review

- [x] ~~Run `/code-review` command~~ — self-reviewed. One new audit op, one
helper, one call site, following the version-purge commands' established wiring.
The property that needed proving — the record is actually emitted by the real
command, not merely emittable — was established by mutation.
- [x] All critical review-responses addressed — none raised.
- [x] All significant review-responses addressed — none raised.
- [x] Self-reviewed the diff for unrelated changes — the guide's Operations table
is a deliberate small extension, explained in the docs checklist.

**Review Responses:** none.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

- **AC1 — the query is recorded. PASS.** Principal, time, verb and entity.
Mutation-verified: removing the call yields `emitted no "acl-query" record`.
- **AC2 — the result is not recorded. PASS**, structurally: the record has no
field in which a result could travel, and the op's godoc says why, citing the
same reasoning `OpACLBypassRead` uses.
- **AC3 — a missing sink is safe. PASS.** Nil-safe with its own test; a reporting
command that failed because nothing was listening would be a worse outcome than
the gap being closed.

**Two deliberate choices worth a reviewer's attention:**

1. **Recorded before the answer.** An attempt to enumerate access for a
non-existent id is as interesting as a successful one; recording only successes
would let a prober stay out of the log.
2. **Not recorded on the no-policy path**, which returns before any attestation
is produced.

## Documentation (enhancements only)

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-L86U7N

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use — `audit.OpACLQuery`'s godoc records
the one decision most likely to be reversed by a well-meaning improvement: why
the answer is not in the record.

## Pull Request

- [x] ~~Run `/pr` command to create PR and monitor CI~~ (deferred by design.)
