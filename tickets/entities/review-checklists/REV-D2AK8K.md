---
id: REV-D2AK8K
type: review-checklist
title: 'Review: Optimistic concurrency at the store: expected-version on entity.Patch / UpdateEntity'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`)
- [x] Lint clean (`just lint`)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] Coverage maintained (`just coverage-check`)

Run after rebasing onto current `origin/develop`, not against the stale base
the branch was written on. `just lint` surfaced four gofmt findings and one
misspell, all introduced by the rebase resolution of the plimsoll rationale
blocks; fixed, and the run is now `0 issues`. `just arch-lint` reports no
warnings and `just plimsoll` passes with the merged method budgets.

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

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-X0TGM4 (significant, addressed), RR-1GM1NB
(significant, addressed), RR-VEACKR (nit, addressed — an audit result, no
change required).

No critical findings. Both significant findings are genuine defects that were
fixed and pinned by failing-first tests:

- **RR-X0TGM4** — `VersionOf` hashed values with `%v`, so `int64(1)`,
`float64(1)` and `"1"` produced the SAME token. A write that changed only a
value's Go type therefore left the version unchanged, and a CAS that should
have conflicted silently succeeded — the exact lost update this ticket exists
to prevent. Fixed by folding `%T` into the hashed field.
- **RR-1GM1NB** — found while rebasing, not by the reviewers. The PATCH
migration passed a bare id to `PatchEntity`; since content-states an id
addresses a state FAMILY, so authorization was decided against the default
face and the write could land on the wrong row. Fixed by passing the fused
state ref.

Self-review of the diff against `origin/develop` found no unrelated changes.
One commit was DROPPED during the rebase: `docs(tickets): webhook receiver
design — TKT-1EM4KL, TKT-EFMRQM`, whose ticket bodies had already landed in
develop with TKT-1EM4KL; its only remaining delta was a status bump belonging
to a different ticket.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

- *A conditional update applies when the version matches, and the returned
version is the post-write one* — PASS.
`TestConformance/CAS/MatchingVersionApplies`, run against all four backends.
- *A stale expected-version is refused rather than silently applied* — PASS.
`TestConformance/CAS/StaleVersionConflicts`, plus
`RetryWithActualSucceeds` for the recovery path.
- *No lost writes under concurrency* — PASS.
`TestConformance/CAS/ConcurrentAppendersAllLandWithRetry` asserts that every
concurrent append SURVIVES, which is the user-visible property rather than a
proxy for it.
- *The suite can actually fail* — PASS by negative control, independently
reproduced: disabling memstore's version comparison fails
`StaleVersionConflicts` and `RetryWithActualSucceeds`; restoring it returns the
package to green.
- *Equal content still hashes equal* — PASS.
`TestVersionOf_StableAcrossInsertionOrder` over repeated rehashes, so the
type-sensitivity fix cannot have been traded for spurious conflicts.
- *The data-entry PATCH path authorizes and writes the face it addresses* —
PASS. `TestFacedIDWrite_AuthorizesTheFaceItWrites` and
`TestFacedIDWrite_ExplicitFaceGrantStillWorks`.

**NOT verified here:** `internal/store/pgstore/cas_crossprocess_test.go` is
gated on `RELA_TEST_DATABASE_URL` and no PostgreSQL instance was available in
this environment, so the cross-PROCESS acceptance test did not run locally. It
is the one case that cannot be demonstrated by the in-process suite. CI runs
the postgres-tagged job, so it is covered before merge, but this checklist
should not claim a local pass it did not get.

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] Docs-checklist created and linked via `has-docs` (DOCS-CAS34X)
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-CAS34X

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI

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
