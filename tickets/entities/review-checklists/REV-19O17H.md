---
id: REV-19O17H
type: review-checklist
title: 'Review: MCP analyze_cardinality: delete the fifth copy, call the consolidated analysis service'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`)
- [x] Lint clean (`just lint`)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] Coverage maintained (`just coverage-check`) — 79.9%, both thresholds PASS

**Comment findings.** `just comment-report` lists the advisory rules
(duplication, nil-contract, param-contract, restatement). They are not a merge
gate, but a finding your diff *introduces* should be fixed or suppressed — don't
grow the backlog.

Every rule is a heuristic over prose, so false positives are expected. To
suppress one, prefer the inline form on the declaration line, which travels with
the code and is reviewed in this diff:

```go
func f(p string) {} //commentlint:ignore param-contract  p is contained by Clone
```

Use `.commentlint.yml` (`ignore:` path globs, `allow-phrases:`) only when the
same prose recurs across many sites. A reason is required either way — an
unexplained suppression is a finding nobody can re-evaluate later.

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent) — plus rela-security-reviewer, since the change touches a read gate
- [x] All critical review-responses addressed — none stand; see note below
- [x] All significant review-responses addressed (RR-GDPBC9, RR-VUSO7Z)
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-16R183, RR-GDPBC9, RR-VUSO7Z, RR-8EB6MF, RR-EVMDXX —
all `addressed`.

**On the one finding rated "critical" by the code reviewer (RR-16R183).**
Downgraded to minor on evidence, and the two reviews disagreed on it: the
security reviewer independently rated the same finding minor and fail-closed.

The critical rating rested on the claim that ACL-gated principals get a WEAKER
cardinality check than before. They do not. The deleted MCP code scanned
`store.EntityQuery{Type: entityType}` — default state only — for EVERY
principal (verified against `develop`). After this change a gated principal on
the pushdown Query branch still scans the prime only, i.e. exactly what it got
before, while an `AllowAll` principal scans every face. The change is a strict
improvement for one class of caller and a no-op for the other; nothing
regressed for anyone, and the failure direction is "violation missed", never
"violation invented".

What WAS wrong is that the godoc I wrote promised per-face coverage
universally. That is corrected in both `handleAnalyzeCardinality` and
`CardinalityReader`. Carrying `AllStates` onto `store.GraphQuery` is a genuine
improvement, but it is a store-layer change touching every backend's query
path — filed as TKT-F9X50Q alongside the face-narrowing work, not smuggled
into a refactor whose contract was "delete the fifth copy".

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

1. **No hand-rolled cardinality scan in `internal/mcp`** — PASS.
   `checkCardinalityBound` and `checkCardinalityForRelation` deleted; the
   handler calls `schema.CheckCardinality`.
2. **A failing `CountRelations` fails the tool call and reports NO
   violations** — PASS.
   `TestHandleAnalyzeCardinality_CountErrorFailsTheToolCall`. Verified to FAIL
   against the pre-change handler, which reported "Found 1 cardinality
   violations" for an entity that genuinely held its edge — the fabricated
   violation the ticket describes, reproduced.
3. **A truncated scan leaves a diagnosable trace** — PASS.
   `TestCheckCardinality_TruncatedScanIsLogged` (schema) asserts the log
   record; `TestHandleAnalyzeCardinality_TruncatedScanStillAnswers` (mcp)
   asserts the tool still answers without fabricating. The log assertion
   failed against the pre-change handler.
4. **MCP and CLI report the same violations and wording** — PASS.
   `TestHandleAnalyzeCardinality_IncomingUsesInverseLabel`, which failed
   against the pre-change handler (`addresses` vs `addressed-by`). Both
   surfaces now render through `CardinalityViolation.Message()`; CLI output
   verified byte-identical to the previous format strings for both the min and
   max branches.
5. **No arch-lint change required** — PASS. `just arch-lint` clean with
   `.go-arch-lint.yml` untouched.

Regression net: the seven existing `TestCheckCardinality_*` tests in
`internal/analysis` (ordering, labels, bound edge cases, multiple source types,
min/max grouping, count-error, per-face counting) pass unchanged against the
moved implementation. Full suite green under `-race`.

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: internal refactor, `kind: refactor`)
- [x] ~~User-facing documentation updated~~ (N/A: no user-facing surface changed; rationale is in godoc)
- [x] ~~Docs-checklist marked as done~~ (N/A: no docs-checklist)

**Docs Checklist:** <!-- e.g., DOCS-xxxx -->

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] ~~Run `/pr` command to create PR and monitor CI~~ (deferred: `/pr` gates on this ticket being `done`, so the PR post-dates this checklist — see the note below and TKT-UFV01M)

<!--
Deliberately NOT tracked here: the PR URL and whether CI passed.

Both post-date this checklist. `/pr` requires the ticket to be `done` and
validating clean before it opens the PR, and a `done` review-checklist may have
no unchecked items — so an item asking for the PR URL can only be satisfied by a
PR that does not exist yet. Checking it early would mean asserting "CI passed"
before CI ran, which turns the checklist from evidence into a formality.

GitHub records both authoritatively, and the branch and commit messages carry
the ticket ID, so the ticket-to-PR link is recoverable without duplicating it
here. See TKT-UFV01M. -->
