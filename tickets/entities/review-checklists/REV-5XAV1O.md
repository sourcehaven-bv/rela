---
id: REV-5XAV1O
type: review-checklist
title: 'Review: Relation writes cannot name the source face'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`go test ./internal/...`, plus `-race` on the touched
packages; pgstore conformance run against a local database since
`RELA_TEST_DATABASE_URL` was unset)
- [x] Lint clean (`golangci-lint` on every touched package, `just arch-lint`,
`just plimsoll`)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] Coverage maintained (`just coverage-check`): package floor and total
thresholds both PASS, total 79.6%

**Comment findings.** Two introduced by this diff, both fixed rather than
suppressed: a duplicated `UpdateRelation` doc paragraph in pgstore, and the
stale `store.RelationData.FromFace` godoc claiming state-tailed update/delete
had no consumer. One `doclink` finding (a bracketed reference to an unexported
method) was fixed by dropping the brackets, which is what Go can actually
render.

One suppression added, with its reason inline: `//nolint:unparam` on
`assertNoEdgeTail`, because its signature deliberately mirrors `assertEdgeTail`
and narrowing one of the pair would make the positive and negative assertions
look like different operations.

## Code Review

- [x] Run `/code-review` command (cranky-code-reviewer **and**
rela-security-reviewer, run in parallel — the change touches ACL, entitymanager
writes, dataentry handlers and all four storage backends)
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-5MLZCR (critical, addressed), RR-OP54MI (significant,
addressed).

The critical finding was mine to have caught: the reconciler derived the tail
from the request address for *every* write, which holds outgoing and is wrong
incoming, where the edge tails at the peer. Clearing an incoming content-scoped
edge from the target side returned 500 and left the link in place. Fixed by
addressing an existing edge by the tail it carries; a new edge is the only case
that takes the tail from the request.

Minor findings also actioned rather than deferred:

- `currentEdgeOnFace` swallowed a store error and returned nil, which
`mergeEdgeMeta` reads as "no prior state" — a transient fault would have erased
the edge's properties. Now returns the error, with absence as
`store.ErrNotFound` so the two are distinguishable.
- `applyRelationsModern`'s `ref` parameter was shadowed by the per-edge
`v1.ResourceIdentifier`. Correct by statement order only; renamed to `addr`.

Two findings were deliberately **not** fixed here and are filed instead:
TKT-0VJ0HV (single-relation routes are default-tail only — incomplete, not
harmful: a faced edge 404s rather than being confused with another) and
TKT-JAROC3, which the review sharpened considerably — relation *restore* takes
the create branch on a faced edge and mints a default-tailed duplicate, forking
the lineage. That is worse than the capture gap it was originally filed for, so
the ticket was rewritten and raised to high.

Self-review: all 25 changed non-ticket files are on the fix path; no TODOs or
FIXMEs introduced.

## Acceptance Verification

- [x] Each acceptance criterion tested
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

- **A faced content-scoped relation is writable** — PASS. The reported
scenario returns 200 where it returned 422; the address the old message
recommended returned 404.
- **The edge lands on the addressed face and nowhere else** — PASS.
`assertEdgeTail` / `assertNoEdgeTail` query by bare id, so they observe where an
edge actually landed.
- **Faces stay isolated** — PASS. Clearing the draft's links leaves the
published face's intact.
- **A symmetric relation via its inverse name tails at the addressed face** —
PASS (the case the deleted guard called out explicitly).
- **An incoming faced edge is addressable from the target side** — PASS,
after RR-5MLZCR.
- **The store contract holds on every backend** — PASS. Three
`UpdateRelationState` cases in the mandatory States suite, green on fsstore,
memstore, sqlitestore and pgstore.

## Documentation (enhancements only)

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: bug fix, no
user-facing surface change — the SPA sends the same request and now gets a 200
instead of a 422)
- [x] ~~User-facing documentation updated~~ (N/A: the worlds manual already
documented the correct model; it was the error message that contradicted it, and
that message is gone)
- [x] ~~Docs-checklist marked as done~~ (N/A)

One operator-facing consequence is worth a release note and is recorded on the
bug: a faced relation write now requires an explicit `type@face` grant, because
`GrantsVerbOnState` treats `*` as covering only the default face. For a faced
type that is a behaviour change, not a no-op.

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] ~~Run `/pr` command to create PR and monitor CI~~ (N/A here: `/pr` gates
on this ticket already being `done` and validating clean, so the PR
necessarily post-dates this checklist — see the note below and TKT-UFV01M.
The branch is `fix/BUG-64MU2Q-faced-relation-writes`.)
