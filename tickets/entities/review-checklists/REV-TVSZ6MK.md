---
id: REV-TVSZ6MK
type: review-checklist
title: 'Review: Store content hash at write time and add a fingerprint column'
started: "2026-11-07"
completed: "2026-11-07"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`) (`go test ./internal/store/... ./internal/sqlitedb/...` and full `just test-postgres` with a schema-pinned DSN pass; the pre-commit hook runs `go test -race ./...`)
- [x] Lint clean (`just lint`) (all tags, 0 issues)
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

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent) (cranky-code-reviewer and rela-security-reviewer)
- [x] All critical review-responses addressed (none raised)
- [x] All significant review-responses addressed (RR-XSV8BAH, RR-CX3JB9Q fixed; RR-5EXMHVP is a step-3 constraint deferred to TKT-IQU106C, no code in this ticket reads the columns out)
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-XSV8BAH, RR-CX3JB9Q, RR-8UHVOJA, RR-VVFOLH1,
RR-USR9S37, RR-NPAFEHU, RR-NI2ORTR, RR-7U1E3SK, RR-FIMCP3P (addressed);
RR-5EXMHVP (deferred to TKT-IQU106C).

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

- No live row has a NULL hash after a write on any database backend: PASS.
`storetest.RunElementTests` checks every row after create, update, an update
with an unchanged hash, rename (with a self-loop), endpoint swap, family
replace, soft delete, restore, face delete and family delete, on pgstore and
sqlitestore. fsstore and memstore have no SQL ranges and are unchanged.
- Conformance covers hash and fingerprint after create, update, rename and
delete: PASS (same suite; it compares the stored hash and limbs with the element
computed from what the store reads back).
- The design supports step 3 range sums: PASS. `TestElementRangeSumUsesIndex`
(pg generic plans use `entities_element_idx` and `relations_element_idx`) and
the sqlite plan check (covering index on both tables).

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated (`.claude/rules/storage.md`; no user-facing surface changes)
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-ZA580R2

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI (run after this checklist is done, per TKT-UFV01M)

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
