---
id: REV-UJ1S7
type: review-checklist
title: 'Review: Enable contextcheck golangci-lint rule'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`) — race-enabled, exit 0, all packages green
- [x] Lint clean (`just lint`) — `0 issues.` with contextcheck enabled (cache cleaned first)
- [x] Coverage maintained (`just coverage-check`) — PASS; total 77.3%, all package floors satisfied

## Code Review

- [x] Run `/code-review` — cranky-code-reviewer invoked on `git diff develop...HEAD`
- [x] All critical review-responses addressed — none found
- [x] All significant review-responses addressed — none found
- [x] Self-reviewed the diff for unrelated changes — pure ctx-threading

**Review Responses:** RR-0VVCI (nit, wont-fix: test-file `context.Background()`
vs `t.Context()`)

Reviewer verdict: "clean, mechanical, correct refactor… Ship it."

## Acceptance Verification

- [x] Each acceptance criterion tested
- [x] Test evidence documented in implementation checklist (IMPL-H250G)

**Acceptance Status:**
- **AC1** — PASS. `.golangci.yml` lists `- contextcheck`, comment block removed.
- **AC2** — PASS. `just lint` → `0 issues.`
- **AC3** — PASS. `just test` exit 0; threaded ctx is the real request/caller ctx.

## Documentation (enhancements only)

- [x] ~~Docs-checklist~~ (N/A: internal refactor + lint-config change)
- [x] ~~User-facing documentation~~ (N/A)
- [x] ~~Docs-checklist marked done~~ (N/A)

## Final Checks

- [x] Commit message explains the why
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` — PR #854 created; rebased onto current develop (resolved conflicts from TKT-9NOX #843 affordances-ctx + TKT-7I3P #840 mcp Deps refactors landed on develop)
- [x] All CI checks pass — Lint, Test, Build, E2E, Frontend, Fuzz, Architecture, Docs, Demos, Rela Tickets, CodeQL, Vulnerability Check all green
- [x] PR URL documented below

**PR:** https://github.com/sourcehaven-bv/rela/pull/854 — MERGEABLE, all checks
green; awaiting reviewer approval (branch protection).
