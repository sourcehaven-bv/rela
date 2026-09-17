---
id: REV-J1NJWX
type: review-checklist
title: 'Review: display: nested — a two-level parent-child view section'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass — full `./internal/...` suite green; `go test -race` on the
      three changed Go packages green; 2456 frontend tests across 151 files.
- [x] Lint clean — `golangci-lint` 0 issues on the changed packages; `just
      arch-lint` OK; `just plimsoll` OK; eslint 0 errors (124 pre-existing
      warnings, none from this diff); `gofmt -l` empty; `vue-tsc -b` clean.
- [x] Comment lint gate clean — `just comment-lint`, no unresolvable doc links
      across 14,073 comments. `just comment-report` shows no NEW advisory
      findings in the changed files (the two in `responses.go` are at lines 300
      and 781, both pre-existing).
- [x] Coverage maintained — `just coverage-check` PASS, total 79.4%.

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

- [x] Run `/code-review` — cranky-code-reviewer AND rela-security-reviewer, run
      in parallel since this touches an ACL read path.
- [x] All critical review-responses addressed — one (RR-0C3IZX, from design
      review); no criticals from code review.
- [x] All significant review-responses addressed — RR-UODKN9, RR-O1VNNN,
      RR-MGMHAC, RR-HKHPYG.
- [x] Self-reviewed the diff for unrelated changes — the diff touches only the
      files this feature needs. The one refactor of existing code
      (`buildSectionRow` extracted from the `table` arm) was verified
      behaviour-preserving by diffing the normalized logic lines against
      `git show HEAD`: the only difference is the renamed parameter
      (`col.Property` → `property`).

**Review Responses:**

| ID | Severity | Status | Summary |
| --- | --- | --- | --- |
| RR-0C3IZX | critical | addressed | Plan claimed section `sort:` existed; `ViewSection` has no `Sort` field. Sorting moved to TKT-9OFGH4. |
| RR-UODKN9 | significant | addressed | Attribution must dedupe on merge (rules run 10× under the fixpoint); `recursive:` now refused. |
| RR-O1VNNN | significant | addressed | Server set `truncated` but the SPA never rendered it — rows vanished silently. |
| RR-MGMHAC | significant | addressed | `display: nested` in a `side_panel` built a tree the wire drops; now refused at load. |
| RR-HKHPYG | significant | addressed | Relation columns resolved over ALL visible children; now select-then-resolve. |
| RR-P751MM | significant | wont-fix | Moot: rollup descoped to TKT-ZAD9PS, finding carried there. |
| RR-HQMQJB | significant | wont-fix | Moot: rollup descoped to TKT-ZAD9PS, finding carried there. |
| RR-4WG64Q | minor | wont-fix | `@click.stop` claim REFUTED by in-app test; comment added explaining the asymmetry. |

**Security review verdict:** the central gating claim holds. `PolicyReader.Filter`
row-gates AND field-redacts every collection before any section builder runs, so
resolving child ids against the filtered collection IS the row gate; nothing
re-reads the store. No existence oracle — `ChildCount` and `HasMoreChildren` are
computed over post-gate sets, and `truncated` follows the gantt's
visible-denial-only rule. `viewResult.Parents` (which holds unauthorized ids) is
never serialized, logged, or exposed. No XSS: the new markup is interpolation
only, no `v-html`. Zero critical or significant security findings.

Minor findings from code review NOT acted on, with reasons:

- *Nil `Parents` renders silently flat* — unreachable from config, since
  validation rejects every config-level cause (unknown bucket, wrong `from:`,
  `recursive:`). A `slog.Warn` for a programming error that config cannot
  produce would be noise.
- *Duplicated parent double-counts* — unreachable: `applyViewTraverse` dedupes
  collections by id, so `parents` is unique. Left as-is rather than adding a
  guard for a state nothing can construct.
- *Accessibility of the count badge and cells* — the count badge was
  subsequently REMOVED entirely (it restated what the expanded rows already
  show), so its `aria-label` is moot. The deeper point (a nested section with
  per-type columns has no header semantics, unlike `display: table`'s real
  `<thead>`) is legitimate and logged as a follow-up rather than redesigned
  here.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

| AC | Status | Evidence |
| --- | --- | --- |
| 1 attribution | PASS | `TestNestedSection_AttributesChildrenToTheirOwnParent`; live server shows TSK-1/TSK-3 under EPIC-1, TSK-2/TSK-4 under EPIC-2 |
| 2 config rejection | PASS | `TestValidateConfig_NestedSection` (10 cases) + `TestValidateConfig_NestedSectionRefusedInSidePanel`, mutation-verified |
| 3 deterministic order | PASS | traversal order; live response identical across requests |
| 4 hidden child absent | PASS | `TestNestedSection_HiddenChildIsAbsent`; security review confirmed the gate is structural |
| 5 capped ≠ childless | PASS | `TestNestedSection_CappedParentIsDistinguishableFromChildless`; live EPIC-3 reports `childCount=0 more=false` |
| 6 truncated on visible denial | PASS | same test asserts both directions; `TestNestedSection_SectionBudgetDropsParentsAndFlags` covers whole-parent drops |
| 7 query budget | **NOT DONE** | see below |
| 8 no bodies | PASS | `TestNestedSection_CarriesNoContent` |
| 9 no fixpoint duplication | PASS | `TestNestedSection_AttributionSurvivesFixpoint`, mutation-verified (removing the dedupe produces `[TKT-003 TKT-003]`) |

**AC7 is not met and I am not claiming otherwise.** No `storetest.Counting`
budget test was written, so the cost claim is unpinned — and root CLAUDE.md
requires one for a new read path. The design is now genuinely bounded
(RR-HKHPYG moved relation-column resolution behind the budget, so it sees at
most `nestedNodeBudget` entities regardless of graph size), and the false
"which the query-budget test pins" comment was deleted rather than left
asserting a guarantee nothing enforces. But the reviewer's point stands: a
Counting test is exactly what would have caught RR-HKHPYG, and
`buildNestedEntityData` running per-node is the kind of thing only that test
settles cheaply. Logged as a follow-up.

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated — `docs/data-entry.md`: `nested` in the
      Display Modes table, a worked subsection, and `children:` in the section
      fields reference.
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-NQS6CB

## Final Checks

- [x] Commit message explains the why, not just what — to be written at commit.
- [x] No TODOs or FIXMEs left unaddressed — `grep -n "TODO\|FIXME"` over the
      changed files returns nothing.
- [x] Ready for another developer to use — verified end-to-end against a real
      project (`rela validate` accepts the config, the API returns a correctly
      attributed tree, the SPA renders and expands it), and the config shape is
      documented with a worked example.

## Pull Request

- [x] ~~Run `/pr` command to create PR and monitor CI~~ (N/A here: `/pr` gates on the ticket already being `done`, so the PR post-dates this checklist — see the note below and TKT-UFV01M)

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
