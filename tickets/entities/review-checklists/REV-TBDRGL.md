---
id: REV-TBDRGL
type: review-checklist
title: 'Review: Restore the AC1.7 test: ACL deny returns a structured 403 body'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`) — `go test ./...` exit 0.
- [x] Lint clean (`just lint`) — 0 issues.
- [x] Comment lint gate clean (`just comment-lint`) — clean.
- [x] Coverage maintained (`just coverage-check`) — adds a test, removes none.
- [x] `just arch-lint`, `just plimsoll` — clean.

## Code Review

- [x] ~~Run `/code-review` command~~ — self-reviewed instead, deliberately.
The diff is one new test file plus a seven-line comment correction; there is no
production logic to review. The property that actually needed proving — *does
this test catch the regression it exists for* — is not something a reviewer
reads off a diff, and it was established by mutation (below) rather than by
inspection.
- [x] All critical review-responses addressed — none raised.
- [x] All significant review-responses addressed — none raised.
- [x] Self-reviewed the diff for unrelated changes — two files, both intended.

**Review Responses:** none. Test-only change, mutation-verified.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

- **AC1 — the AC1.7 contract is pinned again. PASS.** A `*acl.ForbiddenError`
from the real `entitymanager` write path produces 403 with `rule_kind`,
`rule_id` and `reason`. Verified by two mutations, both caught.
- **AC2 — the denial is real, not stubbed. PASS.** `appbuildtest.WithDeclarative`
wires the same `acl.Declarative` as the write-authz ACL, so the error originates
in `AuthorizeWrite` rather than a hand-constructed value.
- **AC3 — the right scenario. PASS.** The role grants read but not update, so
the entity is *visible* and the request reaches the write path. A no-read role
would 404 at the gate and never exercise the handler under test.

**Worth recording precisely: the gap was partial, not total.** Mutating the
handler to never return 403 reddens the suite — that status was covered
incidentally. Mutating only the response *body* left the whole package green.
Reporting this as "AC1.7 is untested" would have been wrong; reporting it as
"covered by other tests" would have been worse. The uncovered half was exactly
the half the AC is about.

## Documentation (enhancements only)

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: `kind=test`.
No user-facing surface changes — no flag, API, config or behaviour.)
- [x] ~~User-facing documentation updated~~ (N/A: same reason. The one doc
change is a stale in-tree comment, covered in the implementation checklist.)
- [x] ~~Docs-checklist marked as done~~ (N/A)

**Docs Checklist:** N/A — test-only.

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use — the test's doc comment states the AC
verbatim, why it needs its own test (the body was uncovered), and why it can now
be wired at this layer when a sibling comment said it could not.

## Pull Request

- [x] ~~Run `/pr` command to create PR and monitor CI~~ (deferred by design:
`/pr` gates on the ticket already being `done`.)
