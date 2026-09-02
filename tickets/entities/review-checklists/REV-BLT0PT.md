---
id: REV-BLT0PT
type: review-checklist
title: 'Review: Regression test for empty FromType against a type-scoped policy'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`) — `go test ./...` exit 0.
- [x] Lint clean (`just lint`) — 0 issues (caught a De Morgan simplification).
- [x] Comment lint gate clean (`just comment-lint`) — clean.
- [x] Coverage maintained (`just coverage-check`) — adds a test, removes none.
- [x] `just arch-lint`, `just plimsoll` — clean.

## Code Review

- [x] ~~Run `/code-review` command~~ — self-reviewed. One new test file, no
production change. The property worth checking — does the table actually test
the ACL as it exists — was established by the test failing first against my
wrong model, and by mutation for the coverage claim.
- [x] All critical review-responses addressed — none raised.
- [x] All significant review-responses addressed — none raised.
- [x] Self-reviewed the diff for unrelated changes — one file.

**Review Responses:** none.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

- **AC1 — never more permissive. PASS**, with one named exemption (below). The
invariant is asserted separately from each row's expectations, so a row with
wrong `want` values still cannot conceal a bypass.
- **AC2 — the realistic grant fails closed. PASS.** `create: [decision]` is
allowed with the source present and **denied** with it absent — the claim stated
positively rather than inferred from an absence of failures.
- **AC3 — exemptions named, not hidden. PASS.** `create: [""]` authorizes an
absent source while denying a present one. Real behaviour, requiring an operator
to write an empty-string grant deliberately. Asserted in **both directions** so
it fails if the exemption stops applying — it cannot rot into a permanent
excuse.

**Two things this ticket got wrong first, both recorded rather than smoothed
over:**

1. The initial table granted the **relation** type and every row failed. The gate
keys on the **source entity's** type. Fixed by reading `authorizeRelationWrite`,
not by adjusting expectations to match a green run — which would have shipped a
test asserting an ACL model that does not exist.
2. A "discriminating case" I added believing it would catch the wildcard
substitution did not, because `grantsVerb` reads the verb's own list. Removed
rather than kept with a comment claiming a property it lacked.

The test's doc now states which bypass shape it **cannot** reach (the wildcard
substitution on a realistic grant needs a client-baseline ceiling, a different
subsystem) so a reader does not over-trust the coverage.

## Documentation (enhancements only)

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: `kind=test`, no
behaviour or surface change. The one piece of new knowledge — the `create: [""]`
exemption — lives in the test's own doc comment, which is where someone auditing
this control will look.)
- [x] ~~User-facing documentation updated~~ (N/A: same reason)
- [x] ~~Docs-checklist marked as done~~ (N/A)

**Docs Checklist:** N/A — test-only.

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use — the test converts an existing prose
guarantee in `authorizeRelationWrite` into a gate, and names both its exemption
and its blind spot.

## Pull Request

- [x] ~~Run `/pr` command to create PR and monitor CI~~ (deferred by design.)
