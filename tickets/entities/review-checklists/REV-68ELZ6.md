---
id: REV-68ELZ6
type: review-checklist
title: 'Review: analysis.faceDeclared treats a bare row as always-declared on a faced type'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] `go test ./...` passes — no failures
- [x] `golangci-lint` clean on the changed packages (a `gocognit` 31/30 on
`CheckStates` was fixed by extracting `coordinateFinding`, not by raising the
threshold)
- [x] `just arch-lint` clean
- [x] `just comment-lint` clean
- [x] `just docs-check` clean — both guides regenerated from their sources
- [x] `just coverage-check` — analysis at 81.8%, both thresholds satisfied

## Code Review

- [x] `/code-review` run (cranky-code-reviewer)
- [x] All critical findings addressed — RR-X7PFKQ, RR-GWPN58, both fixed
- [x] All significant findings addressed — RR-BCUHUU (doc rhetoric, a
falsifiable claim, a third stale copy)

## Verification

- [x] Both critical findings **reproduced before being acted on**, not accepted
on report. A `phantomtype` row did report under `bare-row-on-faced-type`; a
three-type project did collapse into one finding with an empty subject.
- [x] Both fixes mutation-verified. Collapsing `faceUnknownType` into
`faceBareOnFaced` fails the test; re-keying the aggregation on the face fails it
with the exact shipped shape (merged finding, empty subject, `type ""`).
- [x] End-to-end on the real corpus. `prototypes/perf/project` seeded with
`rela dev seed --scale 0.01` now reports two findings — `document: 20 row(s)`
and `policy: 15 row(s)` — where the first draft reported one merged finding
whose five examples were all `DOC-*`, leaving `policy` invisible.
- [x] No in-tree project newly fails: `analyze states` is clean against the
tickets project, docs-project and the worlds prototype.

## Notes

The dangling-colon render fix I committed separately was **reverted**. With a
non-empty Subject on every finding the CLI formatter needs no branch, so the
right fix made the workaround unnecessary. Keeping both would have left dead
code guarding a case that can no longer arise.

One item in RR-BCUHUU was self-inflicted and caught while checking the
reviewer's version of it: my new `unknown-entity-type` sentence claimed no world
could reach a row of an undefined type. Probing `ResolveWorldPrimes` showed it
returns `Via:0` (unscoped, rule 1) — the row **is** served. Same class of error
as RR-Y0UN58 on the previous PR: an absolute claim about world behaviour written
from reasoning rather than from a probe.
