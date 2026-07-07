---
id: REV-1GG5WN
type: review-checklist
title: 'Review: Analysis view: reveal which detail failed a validation (missing headers etc.) on hover/click'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass — `go test ./internal/...` green; frontend `npm run test:run` 1182 pass (16 in AnalyzeView.test.ts incl. new rowKey-collision test)
- [x] Lint clean — `just arch-lint` OK; `golangci-lint` 0 issues on changed pkgs; `vue-tsc` + eslint clean on `AnalyzeView.vue` / `IssuesTable.vue` / `config.ts`
- [x] Coverage maintained — `just coverage-check` PASS (package floor 50% + total 65% both satisfied; total 77.2%). validation 89.9%, validator 83.8%, dataentry 79.9%

## Code Review

- [x] Ran cranky-code-reviewer on commit 130c1970 (diff vs develop)
- [x] All critical review-responses addressed — none raised
- [x] All significant review-responses addressed — none raised
- [x] Self-reviewed the diff for unrelated changes — none (build artifacts gitignored; stray dir removed)

**Review Responses:** Design-review (pre-impl): RR-UFUX7T (critical), RR-ZOGG1X,
RR-M2880C, RR-7UJUAI (significant), RR-WFJQ1I, RR-1GP8NI (minor), RR-WJ65QK
(nit) — all `addressed` in the implementation. Code-review (this phase):
RR-D2BOYL (minor) — rowKey collision for two same-Description rules on one
entity; `addressed` (index folded into key + regression test). The reviewer
raised no critical/significant findings and called the threading "airtight" and
a11y "done right."

## Acceptance Verification

- [x] Each acceptance criterion tested (see planning checklist ACs)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**
- AC1 (expand shows missing headers) — PASS: `MissingRequiredHeaders` unit + `analyze` integration + API round-trip + browser (screenshot: "Missing required headers: ## Oorzaak / ## Corrigerende maatregel"); collapse verified (aria-expanded toggles, detail row removed).
- AC2 (title navigates, message doesn't) — PASS: component test + browser (title → `/entity/ncr/NCR-001`).
- AC3 (satisfied → no row) — PASS: existing analyze tests green.
- AC4 (script-error → dialog from message cell) — PASS: updated component test.
- AC5 (non-content → no detail, omitempty) — PASS: wire test asserts omission.
- AC6 (pattern excluded) — PASS: `content_rule_test.go` case.

## Documentation (enhancements only)

- [x] Docs-checklist created and linked via `has-docs` (DOCS-W71G70)
- [x] User-facing documentation updated — `docs-project/.../GUIDE-data-entry.md` (source) → regenerated `docs/data-entry.md` via `just docs`
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-W71G70

## Final Checks

- [x] Commit message explains the why, not just what (2 commits: feat + review-fix)
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [ ] Run `/pr` command to create PR and monitor CI
- [ ] All CI checks pass
- [ ] PR URL documented below

**PR:** https://github.com/sourcehaven-bv/rela/pull/1100
