---
id: REV-DENHHG
type: review-checklist
title: 'Review: Relation reads and writes mishandle faced endpoints'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`)
- [x] Lint clean (`just lint`)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] Coverage maintained (`just coverage-check`)

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

**Review Responses:** The already-fixed parts of this bug were reviewed per PR
under TKT-2528AB (responses linked to TKT-2528AB, including RR-2IK76Z on
content-tail gating and RR-S4S8ZG on the batched endpoint read; none open). The
commands and gantt fix ran `/code-review` (cranky + rela-security-reviewer):
RR-2L49Q4 (security), RR-ZH9B4J, RR-WT1VVX (significant, addressed), RR-3EPC0W,
RR-EIYCYO, RR-1PVLCS, RR-IQSLZO (minor, addressed), RR-EB9KR7 (minor, deferred
to TKT-KQXVF7), RR-HP33D4, RR-A3CT9F, RR-OD359U, RR-JC6NDA, RR-V0X1DK (nit,
addressed).

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

- `FilterRelations` keeps edges touching faced entities the reader may read: PASS (`TestEndpointsReadable`, `TestPolicyReader_FilterRelationsBudget`).
- Relation create and the relation-target GET to a faced target succeed; a denied face is the uniform 404: PASS (`relation_faces_test.go`).
- Clone and relation writes apply the face gate: PASS (`TestRelationWrites_DeniedFaceIsTheUniformMiss`).
- Command payloads serve a content edge only with its face and never a hidden peer: PASS (`commands_face_test.go`).
- The gantt uses a content edge only under its owning face, and the drill never loads a withheld face: PASS (`gantt_face_test.go`).
- Unlink default tail: moved to Stage 2 (BUG-J3PBFN, TKT-KQXVF7).

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: bug)
- [x] User-facing documentation updated (`docs/acl-security.md`, command permission table)
- [x] ~~Docs-checklist marked as done~~ (N/A: bug)

**Docs Checklist:** <!-- e.g., DOCS-xxxx -->

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] ~~Run `/pr` command to create PR and monitor CI~~ (N/A: the PR is opened after done against faces-intrinsic)

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
