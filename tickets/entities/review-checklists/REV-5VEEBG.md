---
id: REV-5VEEBG
type: review-checklist
title: 'Review: Global `/` search shortcut fires while typing in the Milkdown editor'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass — frontend 2683/2683 (168 files); `go test ./internal/dataentry/...` ok
- [x] Lint clean — `npm run lint` 0 errors (124 warnings, all pre-existing); `just arch-lint` OK
- [x] Comment lint gate clean — `just comment-lint`: no unresolvable doc links across 14921 comments
- [x] ~~Coverage maintained~~ (N/A: `.testcoverage.yml` floors are Go-package thresholds; this diff adds no Go code. The frontend has no coverage enforcement by design — see frontend/CLAUDE.md.)

`vue-tsc --noEmit` also clean. Comment-lint and arch-lint scan `./internal` and
`./cmd`, which this diff does not touch; run and recorded anyway rather than
assumed.

## Code Review

- [x] Run `/code-review` command — cranky-code-reviewer + rela-security-reviewer, in parallel
- [x] All critical review-responses addressed — none raised
- [x] All significant review-responses addressed — 3 of 3
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-SNHEFQ, RR-3IASH2, RR-27HJK4 (significant, all
addressed); RR-E6ZHDZ, RR-FAVKDC (minor, addressed); RR-JF9QWK (nit, wont-fix);
RR-2P1EBB (security nit, wont-fix — not exploitable).

Self-review: `git diff` on the two production files shows exactly the intended
two-line guard swap plus two imports. No unrelated change. Several scratch files
were created during mutation testing and verification; all removed, tree
confirmed clean.

Two reviewer claims were checked rather than accepted, and both needed
correction:

- The security reviewer's forged-`.CodeMirror` attack was tested against
DOMPurify alone. Through the real `renderMarkdown()` pipeline the `.CodeMirror`
wrapper is dropped while the focusable anchor survives, so `closest()` matches
nothing and the guard cannot be forged. Downgraded `minor` → `nit`, closed
`wont-fix` with the verification recorded.
- The cranky reviewer reported a `localName` mutation leaving "all 3
assertions green". Re-running it, that mutation IS caught (assertion 3 —
replacing the guard drops the unused import). The genuine bypass was narrower:
retain the import, bypass the call. That one did pass, and it drove the
per-handler rework in RR-27HJK4.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

| Criterion | Status | Evidence |
|---|---|---|
| `/` does not fire in the Milkdown editor | PASS | Real app, pre/post build comparison; behavioural test mounting Sidebar |
| `/` still reaches `/search` outside a text surface | PASS | Real app; `navigates to /search` test |
| `f` filter shortcut guarded likewise | PASS | SearchView.shortcut.test.ts |
| List views keep owning their own search box | PASS | TKT-603FQ deferral test |
| Escape / Cmd+Enter handlers stay unguarded | PASS | Five existing handlers verified correct; scan scoped to bare printable keys |

Full evidence table (including the pre-fix reproduction in the running app) is
in IMPL-O7F1T8.

## Documentation (enhancements only)

Skipped: this is a bug fix with no user-facing surface change. The shortcut
behaves as `KeyboardShortcutsModal.vue` and FEAT-Q767 already document it — the
fix closes a gap between documented and actual behaviour rather than changing
the contract.

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: bug fix)
- [x] ~~User-facing documentation updated~~ (N/A: no contract change)
- [x] ~~Docs-checklist marked as done~~ (N/A: bug fix)

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

One deliberate limitation is recorded in the scan's docstring rather than
hidden: regex-over-source is a heuristic, and an ESLint `no-restricted-syntax`
AST selector is the immune long-term answer. That is noted as follow-up, not
silently omitted.

## Pull Request

- [x] ~~Run `/pr` command to create PR and monitor CI~~ (done-before-PR gate:
`/pr` requires the bug to be `done` first, so the PR necessarily post-dates this
checklist — see TKT-UFV01M and the template note below. GitHub records the PR
and CI authoritatively; the branch and commit messages carry the bug ID.)

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
