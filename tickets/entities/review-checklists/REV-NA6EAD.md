---
id: REV-NA6EAD
type: review-checklist
title: 'Review: Type picker and fuzzy ranking for the editor''s @ mention completion menu'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass — frontend **2840 unit** tests in 173 files; **303 e2e
passed / 10 skipped / 0 failed**. `just test` (Go) not re-run: this diff
contains **zero Go files**, so no Go test can be affected by it.
- [x] Lint clean — `npm run lint` **0 errors**, and **zero warnings in any file
this ticket touches**. Total warnings 129 → 128 against baseline: the change
removed a pre-existing `max-lines` warning by extracting `mentionKeymap.ts`.
`npm run typecheck` clean.
- [x] Comment lint gate clean (`just comment-lint`) — "no unresolvable doc links
across 15007 comments". It scans `./internal ./cmd` only, so it is unaffected
either way by a frontend-only diff.
- [x] ~~Coverage maintained (`just coverage-check`)~~ (N/A: **zero Go files
changed**, so the Go floors cannot move; the frontend has no coverage
enforcement by project policy — see frontend/CLAUDE.md. Two attempts were killed
by signal 15 partway through the race-enabled run; recorded as not-run rather
than claimed as passing.)

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

- [x] Run `/code-review` command — cranky-code-reviewer **and**
rela-security-reviewer, run in parallel, since the change touches an
ACL-adjacent read path.
- [x] All critical review-responses addressed — 4 of 4.
- [x] All significant review-responses addressed — 4 of 4.
- [x] Self-reviewed the diff for unrelated changes — one deliberate two-line
drive-by: a doc comment above `tableCommands` that actually described
`ALL_COMMANDS` was reattached to it. Nothing else unrelated.

**Review Responses:** 11 findings, all addressed.

*Critical (4), all one root cause — an index-based highlight against rows that
change asynchronously:* RR-J3OEGA (stranded index → Enter inserts a paragraph
break into the document), RR-T6SNQN (shrinking type list slides the highlight
onto another row → Enter inserts the wrong entity), RR-0GQVYU (`selectType`
leaves the highlight on a pre-scope entity), RR-XWOQZH (previous scope's rows
stay under the new chip). Fixed structurally: the highlight stores an IDENTITY
and resolves it against the live rows, making a stale index unrepresentable.

*Significant (4):* RR-8I7XUD (`intraIns: 1` → 55 s main-thread freeze, measured;
removed), RR-G3YZ8I (a partly-tokenizable query silently drops server matches —
and affects accented Latin, not only CJK), RR-FT7A57 (schema reload repopulated
a closed menu; `dispose` guard was missing on `setAvailableTypes`), RR-6A9A8P
(the test suite could not observe the pre-response window where the criticals
lived).

*Minor (2):* RR-ARZBY9 (one `role=listbox` with `aria-activedescendant`
replacing two nested listboxes), RR-83NRB1 (duplicate keys, redundant request,
error-note attribution, per-row recomputation).

*Nit (1):* RR-N8M7F4 — slice-order test and shared guard done; `fast-check`
property tests deliberately deferred rather than add a dependency in this
ticket.

Every fix is **mutation-verified**: reverting it fails the test written for it.
That check earned its keep — the stranded-index test did NOT fail under mutation
at first, and was strengthened (highlight the LAST entity, not the first) until
it did.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:** all 10 PASS. The per-AC table with evidence is in
IMPL-G2V20X. Highlights: AC 7 (a prefix must never blank the menu) is pinned by
a 13-step matrix over every prefix of `fancy-ranking`; AC 3 (Backspace clears
the chip) is an e2e test with no waits between keystrokes, because any pause
hides the staleness bug it guards; AC 10 (never invents rows) is asserted over 7
query shapes.

Two ACs were verified in a real browser rather than jsdom because jsdom cannot
observe them at all: the ProseMirror update-cycle staleness (AC 3) and the
`aria-activedescendant` wiring added for RR-ARZBY9.

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated — `docs/data-entry.md` § The Markdown
Body Editor: fuzzy matching across title and ID, the type picker and its
progressive disclosure, how to scope and how to clear the chip, and that a space
still closes the menu. `frontend/CLAUDE.md` records the invariants a future
change could silently break (query sent unmodified, rank-don't-filter, the
tokenizer guard, uFuzzy defaults, and the identity-based highlight).
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-YX1IC8

## Final Checks

- [x] Commit message explains the why, not just what — names the two defects the
change fixes, why type inference was abandoned (with the measured ambiguity), and
the three load-bearing invariants a future change could undo.
- [x] No TODOs or FIXMEs left unaddressed — grepped the new and modified sources;
no TODO/FIXME/XXX/`console.log`/`debugger`.
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI — done. GitHub CI is green
(29 pass, 2 skipping, 0 failures), including CodeQL after the `useId()` fix, and
auto-merge is armed pending review. Per the note below the URL and CI status are
not recorded as checklist evidence; they post-date this checklist.

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
