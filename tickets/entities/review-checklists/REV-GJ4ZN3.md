---
id: REV-GJ4ZN3
type: review-checklist
title: 'Review: idp-sync example: validate webhook claims before interpolating them'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`) — `go test ./...` exit 0 (unaffected: no Go
code changed).
- [x] Lint clean (`just lint`) — 0 issues.
- [x] Comment lint gate clean (`just comment-lint`) — clean.
- [x] Coverage maintained (`just coverage-check`) — no Go code changed.
- [x] `just arch-lint`, `just plimsoll`, `just lint-md` — clean.

Note the repo has **no linter or test harness for `examples/`**, so none of the
above actually exercises this change. That is why the pattern was executed
against a real `lua` interpreter — see the implementation checklist. Worth
knowing when reviewing: the gates being green says nothing about this diff.

## Code Review

- [x] ~~Run `/code-review` command~~ — self-reviewed. Five lines of Lua in an
example script; the only thing worth checking is whether the pattern is right,
and that was established by running it over a twelve-case table rather than by
reading it.
- [x] All critical review-responses addressed — none raised.
- [x] All significant review-responses addressed — none raised.
- [x] Self-reviewed the diff for unrelated changes — one file.

**Review Responses:** none.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

- **AC1 — dangerous characters rejected. PASS.** `../etc/passwd`, `a/b`, `a?b`,
`a#b`, `a b`, `a\nb` all rejected.
- **AC2 — ordinary identifiers accepted. PASS.** Emails, slugs, ULIDs, uppercase
ids all accepted.
- **AC3 — existing error shape. PASS.** Returns
`{ message_type = "error", message = ... }`, matching the two guard clauses
already above it.

**One deliberate rejection is worth flagging to a reviewer:** `auth0|abc123`, a
common real subject format, does **not** pass. That is intentional — `|` is
harmless in a path, but adding characters because they happen to be safe today
is how an allowlist decays into a blocklist. The comment names the case and says
to widen the pattern character by character if your IdP needs it.

## Documentation (enhancements only)

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: `kind=docs`, and
the change *is* documentation — the comment in the example is the deliverable as
much as the five lines of code.)
- [x] User-facing documentation updated — the comment states why the check
exists, that it is defence in depth rather than the primary control (so nobody
relaxes the JWT verification), and how to widen it safely.
- [x] ~~Docs-checklist marked as done~~ (N/A)

**Docs Checklist:** N/A — the example's own comment.

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use — which for an example is the whole
point: the next person copies this file, and now copies the guard with it.

## Pull Request

- [x] ~~Run `/pr` command to create PR and monitor CI~~ (deferred by design.)
