---
id: IMPL-Q8S5AF
type: implementation-checklist
title: 'Implementation: Face migration is not atomic and rename does not validate face sets'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code — one per defect, each written to fail
against the unfixed code first.
- [x] ~~Integration tests~~ (N/A: both tests drive the real migration runner
over a real store, which is the full flow for a CLI-only surface.)
- [x] Happy path implemented — moves batch through `store.Store.Tx`; a rename
into a divergent face set is refused at `Validate`.
- [x] Edge cases handled — `applyFaceMove` keeps the existing idempotency
contract (a destination row with identical content is the previous run's copy;
anything else is a refused collision), which is what makes fs/mem's no-rollback
tier recoverable by re-run.
- [x] Error handling in place — a batch error aborts the step and propagates
unchanged; the rename refusal names the orphaned faces and the remedy steps.

## Test Quality

- [x] Using fixture builders — reuses `seedStore`, `metaV1`, `facedV1`,
`newTestRunner`, `mustParse`.
- [x] No hardcoded values in assertions when the object is in scope
- [x] Only specifying values that matter — the rename fixture differs from the
baseline in exactly one dimension, the declared face set.
- [x] ~~Interpolated values constructed from objects~~ (N/A: the rename test
asserts the error names operator-facing identifiers, which are literals of its
own fixture by design.)
- [x] Property comparisons use the error text and the probe's own counters, not
golden output.

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified
- [x] Edge cases manually verified

**Verification Evidence:**

- **Both defects reproduced before being fixed.** This bug was filed unverified
from a review of an adjacent change, so the first step was proving it real:
  - `Validate` accepted a rename from a type declaring `draft`/`published` into
one declaring only `live`.
  - A store wrapper recording transaction context showed all 6 writes (3 moves ×
create+delete) reaching the outer store directly.

- **Both fixes mutation-verified.** Reverting the apply loop to its pre-fix
shape fails `TestMigrateFace_MovesRunInsideATransaction` with "6 write(s)
reached the store outside a transaction". Removing the face-set comparison fails
`TestRenameEntityType_RefusesDivergentFaceSets`.

- **Backend limits established from the contract, not assumed.**
`store.Store.Tx` gives rollback on pg/sqlite and serialization only on fs/mem.
The atomicity fix is therefore **partial on fs/mem**, where idempotent re-run
stays the recovery path. That is stated in the code comment rather than glossed,
because a reader who assumes uniform atomicity would be wrong.

- Full suite green: `go test ./...` no failures, `arch-lint`, `comment-lint`,
`golangci-lint` all clean.

## Quality

- [x] Code follows project patterns — the batching mirrors `forEachEntity`'s
existing `updateBatchSize` loop rather than inventing a second shape, and reuses
its constant for the same documented reason (a graph-wide transaction stalls
every writer on pg).
- [x] Checked for DRY opportunities — `applyFaceMove` was extracted because the
loop body now runs inside a closure, and the extraction puts the transaction
view in the signature where the "never write through the outer store" rule is
visible. Not extracted for its own sake.
- [x] No security issues introduced — operator-shell trust boundary, no ACL
path, and the rename change only ever refuses more.
- [x] No silent failures — the rename now fails loudly at load where it
previously stranded rows quietly.
- [x] No debug code left behind.
