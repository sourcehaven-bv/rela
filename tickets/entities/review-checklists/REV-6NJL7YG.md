---
id: REV-6NJL7YG
type: review-checklist
title: 'Review: Declarative LLM actions: llm: block on data-entry actions'
started: "2026-11-13"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass: Go tests of every touched package (`dataentry`, `llmtask`, `dataentryconfig`, `archguard`, `mcp`, `script`, `ai`, `lua`, `conditionlint`) and all 6177 frontend unit tests; the full suite runs in CI
- [x] Lint clean: golangci-lint on the changed packages (0 issues), `just arch-lint`, `just plimsoll`, frontend typecheck and ESLint
- [x] Comment lint gate clean (`just comment-lint`: no unresolvable doc links)
- [x] Coverage maintained: `just coverage-check` passed before the review fixes; the fixes only add tests

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

- [x] Ran `/code-review` with cranky-code-reviewer and rela-security-reviewer
- [x] All critical review-responses addressed (none were raised)
- [x] All significant review-responses addressed: dialog navigation race, metamodel value rules, gated header candidate read, missing handler tests
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-0S8LTN1 RR-13MJ1TP RR-19ED3W9 RR-29GZ2TA RR-2OGP0MZ RR-4YHEPAK RR-5H3ME4L RR-5KLG13U RR-77ZJCOX RR-7L3ZDS8 RR-9CDOFLM RR-E7Y7TO8 RR-FIM4AWO RR-FQI9G9S RR-G15T2RT RR-GINXNTD RR-HYAMBG2 RR-N1B15K1 RR-QPW5ADO RR-WM4DSW7 RR-WNQA13C RR-WVZPW7J RR-X6U21TH RR-ZXPYDQJ. One minor (RR-ZXPYDQJ, max ignores existing relations) is deferred with a reason; the rest are addressed.

## Acceptance Verification

- [x] Each acceptance criterion tested (see PLAN-ZKSW8KB)
- [x] Test evidence documented in IMPL-67HZE4G

**Acceptance Status:**

- Config validation of `llm:` (inputs, outputs, permission, limits, elevation refused): PASS, `dataentryconfig` tests.
- Gated, redacted inputs and candidates: PASS, `TestLLMAction_RelatedInputs`, `TestLLMAction_Candidates`, budget test `TestLLMAction_CandidateReadBudget` (10 reads at 10 and 50 rows).
- Schema-derived structured output, checked again on reply: PASS, `llmtask` tests and `TestLLMAction_ReplyInvalid`.
- Proposal only, no write by the action: PASS, handler tests and manual run against Ollama.
- Dialog applies only the ticked changes, skips stale properties, drops a proposal for a left entity: PASS, `LLMProposalDialog.test.ts`, `EntityDetail.actions.test.ts`, manual browser run.

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated (`GUIDE-data-entry.md`, `GUIDE-lua-scripting.md`, `GUIDE-mcp-server.md`; `just docs`)
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-2YPHMOD

## Final Checks

- [x] ~~Commit message explains the why~~ (N/A: not committed yet; the user decides when to commit)
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] ~~Run `/pr` command~~ (N/A: the user decides when to open a PR)

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
