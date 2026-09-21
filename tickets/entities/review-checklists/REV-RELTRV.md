---
id: REV-RELTRV
type: review-checklist
title: 'Review: relation traversal in conditions'
status: done
---

## Automated Checks

- [x] All tests pass — default, `postgres` and `sqlite` build tags all green
- [x] Lint clean (`just lint` — 0 issues)
- [x] Comment lint gate clean (`just comment-lint` — no unresolvable doc links
      across 15269 comments; one `doclink` finding this diff introduced was
      FIXED, not suppressed: `[Program.inspect]` is unexported and Go cannot
      link it, so the brackets were dropped)
- [x] Coverage maintained (`just coverage-check` — package 50% PASS, total 65%
      PASS, total 79.7%)

**Comment findings.** The diff introduced exactly one advisory finding and it
was fixed rather than suppressed. No suppressions were added.

## Code Review

- [x] Run `/code-review` — cranky-code-reviewer and rela-security-reviewer, in
      parallel, on the merge-base diff
- [x] All critical review-responses addressed (RR-TRV03)
- [x] All significant review-responses addressed (RR-TRV01, RR-TRV02, RR-TRV04)
- [x] Self-reviewed the diff for unrelated changes — every changed Go file is
      traversal-related; no drive-by edits

**Review Responses:** RR-TRV01, RR-TRV02, RR-TRV03, RR-TRV04, RR-TRV05,
RR-TRV06 — all `addressed`.

Three were found by the security reviewer and two of those were defects I did
not spot: the `visible:` CLOSED WORLD (a field hidden by omission has no
`when:` to detect, and that is the likelier operator spelling) and the missing
per-principal client ceiling. RR-TRV03 was a defect I INTRODUCED while fixing
an unbounded-recursion finding mid-review — rendering a too-deep arm as
`AND FALSE` inverts under an outer `Negate` to "match every row", turning a
DoS guard into a row-gate bypass.

## Acceptance Verification

- [x] Each acceptance criterion tested
- [x] Test evidence documented in IMPL-RELTRV

**Acceptance Status:**

- *Traversal filters on a related entity's properties* — PASS.
  `storetest.RunEndpointMatchTests`, 9 scenarios, green on fsstore, memstore,
  sqlitestore and pgstore.
- *Pushed down to SQL, not per-row* — PASS. pgstore emits one query; the
  EXPLAIN test asserts the plan uses the derived index on the traversed-to
  type, reconciling the spec `queryplan.TraversalIndexSpecs` DERIVES rather
  than a hand-written one, so derivation and lowering cannot drift.
- *Union targets require a `type=` ascription* — PASS against the REAL
  `tickets/schema.yaml`: the bare form is refused naming all four candidate
  types; the ascribed form is accepted.
- *Untrusted traversals are gated* — PASS. Row gate per hop, field gate
  (conditional AND closed-world), per-principal ceiling, and three
  fail-closed refusals for shapes `EndpointPredicate` cannot express
  (face-restricted read, disjunctive `Any`, nested inheritance expansion).
- *Negative controls* — PASS. Tests verified to FAIL when the guard they pin
  is removed: ACL row gate, client ceiling, slice-copy, and the EXPLAIN
  index guard. A security test that cannot fail proves nothing.

## Documentation (enhancements only)

- [x] Docs-checklist created and linked via `has-docs` (DOCS-RELTRV)
- [x] ~~User-facing documentation updated~~ (N/A: no user-facing surface yet —
      nothing calls the gate or the validator, so `related(...)` is not
      writable in any operator-edited config. See DOCS-RELTRV.)
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-RELTRV

## Final Checks

- [x] Commit messages explain the why, not just the what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

**Known gaps, deliberate and recorded:** the feature is UNWIRED — nothing calls
`GateTraversal` or `ValidateTraversals`, which caps every review finding at
`significant` (no live exploit) and is why docs are deferred. Wiring it to a
live surface (view `where:`, `--filter`) is the follow-up. Ordered comparison,
"all" semantics, and traversal on affordance surfaces are v2. SQLite executes
traversal through the Go path; that cost is quantified and filed as TKT-SQLGQ.

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI
