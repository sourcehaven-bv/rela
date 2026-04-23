---
id: REV-8B3QP
type: review-checklist
title: 'Review: Refactor document links to app-relative + add Lua router/URL helpers'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`) — confirmed via `just check`; all 40+ packages green
- [x] Lint clean (`just lint`) — 0 issues
- [x] Coverage maintained (`just coverage-check`) — total 73.9%, above 65% floor

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent) — plus parallel go-architect review
- [x] All critical review-responses addressed (RR-0GPDV, RR-CKLD2)
- [x] All significant review-responses addressed (RR-T0MGE, RR-B0J38, RR-JZ0T6)
- [x] Self-reviewed the diff for unrelated changes — only TKT-UUHVT.md was unrelated and deliberately excluded from the commit

**Review Responses:**

17 total (all linked via `has-review-response`):

Critical (2, addressed):
- RR-0GPDV stale e2e test
- RR-CKLD2 return_to collision

Significant (3, addressed):
- RR-T0MGE goldmark entity test fidelity
- RR-B0J38 empty returnPath dangles return_to=
- RR-JZ0T6 rela.url wired too broadly; script→frontendroutes layering

Minor addressed (5): RR-5JNVE stale comments, RR-60UUA mutable var, RR-WVX2X
All() shallow copy doc, RR-PKJTP patternMatches dup, RR-ZBRWR unused
MatchedRoute.Values

Nit addressed (2): RR-UQ73Q dead nil-check, RR-R09JW encoded-slash doc

Minor deferred (3, with documented reasons): RR-5VWLH presence-only query keys,
RR-14UMJ escape asymmetry, RR-8V0E2 parity regex strictness

Nit deferred (2): RR-TOX6O CLI dispatch style, RR-8Y3Z7 frontendparity package
shape

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist PLAN-3E5HR)
- [x] Test evidence documented in implementation checklist IMPL-BK2GI

**Acceptance Status:**

All 12 acceptance criteria from PLAN-3E5HR pass. Evidence listed in IMPL-BK2GI's
"Verification Evidence" section: 23 rewriter subtests, 16 Lua url tests, 13
catalogue tests, 4 CLI subcommand tests, parity test, e2e updated.

## Documentation (enhancements only)

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: docs updates bundled with implementation — GUIDE-data-entry.md rewritten in-ticket, no separate docs phase needed)
- [x] User-facing documentation updated — `docs-project/entities/guides/GUIDE-data-entry.md` section on document links replaced; `docs/data-entry.md` regenerated via `just docs`; `CLAUDE.md` architecture table updated to include `internal/frontendroutes`
- [x] ~~Docs-checklist marked as done~~ (N/A as above)

**Docs Checklist:** none created (see above)

## Final Checks

- [x] Commit message explains the why, not just what — single commit summary with rationale for each major shape decision
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use — `rela.url` + `rela-server routes` both documented with examples

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI
- [x] All CI checks pass — Rela Tickets gate will clear on transition to `done` (it gates `review` status itself)
- [x] PR URL documented below

**PR:** https://github.com/sourcehaven-bv/rela/pull/561
