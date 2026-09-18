---
id: REV-IGC48D
type: review-checklist
title: 'Review: Validation relation gates: a consumer-side graph seam, with direction and target-type filters'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`)
- [x] Lint clean (`just lint`)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] Coverage maintained (`just coverage-check`)

`go test ./internal/...` — no failures. `just lint` exit 0; `golangci-lint`
scoped to the six changed packages reports **0 issues** (it reported 32 before
the review fixes, which would have failed CI). `just comment-lint` gate clean:
no unresolvable doc links across 15178 comments. `just coverage-check` exit 0;
the changed packages measure validationgraph 90.9%, validation 88.8%, metamodel
87.5%, validator 80.5% — all well above the 50% default floor. `just arch-lint`
clean, and `just plimsoll` clean.

**Comment findings.** `just comment-report` surfaces no advisory findings in any
file this diff adds. No suppressions were needed, so none were added.

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

Two reviews ran: the general code review and a security review (this touches the
ACL read path). **No critical findings.** Four significant and several minor;
all significant are fixed.

**Review Responses:**

Design review (pre-implementation) — RR-CE7JVU, RR-WXZ00T, RR-YTRVNW, RR-MSLJHN
(critical); RR-CNM4HB, RR-M2XRW2, RR-WCH0E6, RR-LS7AG1 (significant). All
addressed in the plan before any code was written.

Security review — RR-5F2OPI, RR-SX6813 (minor, both fixed).

Code review — RR-X7JUMF, RR-H0JPZ9, RR-RY4Y0A, RR-N95ULU (significant, all
fixed); RR-Z7S5HZ (minor, fixed); RR-SZJZX9, RR-5I1M63 (minor, deferred with
reasons).

The finding worth naming: **RR-X7JUMF was my own regression.** The seam resolved
every far entity eagerly, so the two bare `min: 1` gates among the 14 shipped
ones did one entity read per edge where they previously did none — a per-row
read on a whole-graph scan, which is exactly what this ticket's own Cost section
said not to do. Counts stayed correct, so no behavioural test could see it.
Fixed by letting the caller decline the read (`resolveFar`), and pinned by a
counting-reader budget test rather than left to be noticed.

Two deferrals, both with reasons rather than silence: one direction vocabulary
shared across three packages (a cross-boundary refactor, and the hazard is
already closed by load-time rejection), and load-time operator/type checking for
`where` (a pre-existing property of every clause on every shipped gate, not
something this change introduces).

Self-review: the diff contains no unrelated changes. Three `zz_*_test.go`
scratch files left behind by review agents were removed. The one adjacent change
— hoisting `internal/filter`'s operator table to a package var and exporting
`Operators()` — exists only to support the drift guard the security review asked
for, and is noted in its commit.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:** all criteria PASS except two recorded N/A with reasons
(verified, not assumed: nothing in the change reads or branches on a face, and
no code path dedupes edges). Full evidence is in IMPL-I3IVIH; the headline
items:

- *Seam* — PASS. `internal/validation` reaches the graph through one
interface it declares; `lua.ReadDeps.OutgoingRelations` deleted outright.
`arch-lint` confirms validation still does not import `internal/store`.
- *No verdict change* — PASS. `rela analyze validations` over `tickets/`
byte-identical with ticket data held constant, re-checked after every commit.
- *Direction* — PASS, and mutation-tested. Reverting the adapter's query to
`{From, Incoming}` fails exactly the two incoming subtests; a test asserting
only "which endpoint did we read" would have passed against that bug.
- *target_type* — PASS, including the case a bare count cannot express (an
edge exists but from the wrong type).
- *Load-time guards* — PASS, all four triggered deliberately in a real
project and the messages copied into the docs.
- *Atlas rules* — PASS end to end in a real project at `severity: error`.

## Documentation (enhancements only)

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-JQAWR8

`docs/metamodel.md` and its `docs-project` mirror both updated. The checklist
records one honest gap: the documented example carries `faces: [vastgesteld]`,
and the project I verified against is faceless because `rela create` cannot yet
name a face (TKT-2RQMV4). That line was reasoned about rather than executed, and
the checklist says so rather than letting a reader assume otherwise.

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

Six functional commits, each explaining the failure mode it prevents rather than
the lines it changed. The seam commit and the keys commit are separate on
purpose, so a regression in the behaviour-preserving move is visible before any
feature sits on top of it.

No TODOs or FIXMEs added. The two deferred findings live on review-response
entities with reasons, not as comments in the code.

## Pull Request

- [ ] Run `/pr` command to create PR and monitor CI

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
