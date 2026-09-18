---
id: REV-VHEGPE
type: review-checklist
title: 'Review: Face migration is not atomic and rename does not validate face sets'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] `go test ./...` passes — no failures
- [x] `golangci-lint` clean (a `govet` shadow introduced by reading
`DeleteResult` was fixed, not suppressed)
- [x] `just arch-lint` clean
- [x] `just comment-lint` clean
- [x] `just docs-check` clean
- [x] `just coverage-check` — datamigration at 70.7%, both thresholds satisfied

## Code Review

- [x] `/code-review` run (cranky-code-reviewer)
- [x] All critical findings addressed — RR-4R3F4K (generator), RR-GQ8Y83 (edge
loss)
- [x] All significant/minor findings addressed — RR-8T3RKW

## Verification

- [x] Every finding reproduced before being acted on. The generator failure was
confirmed verbatim (`draft=false`, "generated draft does not parse (generator
bug)"); the edge loss was confirmed against the existing fixture (1 relation
before, 0 after).
- [x] Both fixes mutation-verified. Removing the generator branch reproduces the
exact operator-facing failure; reverting either apply loop reports writes
outside a transaction.
- [x] Batch boundary pinned at 0, 1, `updateBatchSize`, +1 and ×2.

## Scope grew, deliberately

Two things landed that the ticket did not name:

**`rename_face` had the identical defect.** The ticket named `migrate_face`
only. `rename_face` carries the same create+delete pair, and its own comment
described the crash-between-the-two it could not prevent. Fixing one and not its
twin would have contradicted this bug's own why5 — the two steps differ in which
coordinate a row moves *from*, never in what a half-applied move leaves behind.
Both now share one `applyMoves` loop.

**The edge loss (RR-GQ8Y83) is a worse bug than the one filed.** It is
pre-existing, destroys data rather than risking a transient split, and I would
not have found it without the reviewer's version-capture question. Fixed here
because it lives in the exact function this ticket rewrites; splitting it out
would have meant touching `applyFaceMove` twice.

## Note

The reviewer's tree went stale mid-run: it reported `rename_face` as unfixed and
its own `git checkout` reverted work in progress. Finding 1 was already done and
had to be re-applied. Worth knowing for the next concurrent review — the code
under review should be committed, not left in the working tree.
