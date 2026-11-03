---
id: REV-FN2NTU
type: review-checklist
title: 'Review: File properties show the file name in table and list cells'
started: "2026-10-16"
completed: "2026-10-16"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`) (frontend only change: `npm run test:run` passes, 308 files; `npm run typecheck` clean; no Go files changed)
- [x] Lint clean (`just lint`) (`npm run lint`: 0 errors, no new warnings; prettier clean on new files; pre-commit hooks pass)
- [x] ~~Comment lint gate clean (`just comment-lint`)~~ (N/A: no Go changes)
- [x] ~~Coverage maintained (`just coverage-check`)~~ (N/A: Go coverage only; the frontend has no coverage gate)

**Comment findings.** `just comment-report` lists the advisory rules
(duplication, nil-contract, param-contract, restatement). They are not a
merge gate, but a finding your diff *introduces* should be fixed or
suppressed — don't grow the backlog.

Every rule is a heuristic over prose, so false positives are expected. To
suppress one, prefer the inline form on the declaration line, which travels
with the code and is reviewed in this diff:

```go
func f(p string) {} //commentlint:ignore param-contract  p is contained by Clone
```

Use `.commentlint.yml` (`ignore:` path globs, `allow-phrases:`) only when the
same prose recurs across many sites. A reason is required either way — an
unexplained suppression is a finding nobody can re-evaluate later.

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent) (cranky-code-reviewer on the full diff)
- [x] All critical review-responses addressed (none)
- [x] All significant review-responses addressed (RR-1EDQPH, RR-3XUAR2)
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-1EDQPH, RR-3XUAR2 (significant, addressed);
RR-MNO99X, RR-WOASY8, RR-ISSYWT, RR-IO4W1J, RR-NNNUMG (minor, addressed);
RR-F81DTA, RR-3IIMIR (nit, wont-fix).

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist) (PLAN-4O7K1U)
- [x] Test evidence documented in implementation checklist (IMPL-2W3QJH)

**Acceptance Status:**
1. PASS: one file shows paperclip and name, linked (EntityDetail.table.test.ts "shows one file as its name"; manual check on the atlas demo).
2. PASS: "N files" summary with names in title and list ("summarizes several files").
3. PASS: empty cell blank ("leaves an empty file cell blank").
4. PASS: ellipsis via CSS, full name in title (FileCellWidget.test.ts).
5. PASS: face address ("links to the face the row was served in").

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-ZOJUAH

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI
