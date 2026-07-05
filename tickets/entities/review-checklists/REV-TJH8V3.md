---
id: REV-TJH8V3
type: review-checklist
title: 'Review: Restore relation filter controls on the v1 list pipeline + add direction:incoming'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`) — verified via PR #1065 CI: `Test` job green.
- [x] Lint clean (`just lint`) — PR #1065 CI: `Lint` + `God-object lint` green.
- [x] Coverage maintained (`just coverage-check`) — PR #1065 CI: `Test` job includes coverage gate; green. New tests added: `relation_filter_test.go` (+446), `config_test.go` (+98), `validate_test.go` (+187).

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent) — produced 6 review-responses.
- [x] All critical review-responses addressed — RR-6RF60V (critical, fail-open operators) status=addressed.
- [x] All significant review-responses addressed — RR-0HWAS0, RR-38K7K9, RR-9MJRJG, RR-B0JPPL, RR-HK1XNO all status=addressed.
- [x] Self-reviewed the diff for unrelated changes — diff scoped to relation-filter path (`api_v1.go`, `config.go`, `validate.go`, tests); dead `filterByRelation`/`resolveRelationFilterValues` + their orphaned tests removed.

**Review Responses:** RR-6RF60V (critical), RR-0HWAS0, RR-38K7K9, RR-9MJRJG, RR-B0JPPL, RR-HK1XNO (significant) — all addressed.

## Acceptance Verification

- [x] Each acceptance criterion tested — covered by the automated suite (`relation_filter_test.go`), green in CI. NOTE: verified via the CI test suite, not manually re-run by the reviewer completing this checklist.
- [x] Test evidence documented — see below.

**Acceptance Status:**
1. Incoming relation filter (`filter[verantwoordelijk_voor]=Jeroen Vloothuis`, direction: incoming) narrows the taak list to tasks whose incoming source titles to Jeroen — PASS (covered by `relation_filter_test.go`).
2. Outgoing relation filter (default direction) works — PASS (covered by `relation_filter_test.go`).
3. Scope prev/next navigation over a relation-filtered list matches list ordering — PASS (`_position`/scope test in `relation_filter_test.go`).

Evidence basis: PR #1065 `Test` CI job green over the added `relation_filter_test.go` cases. Not independently re-run by the agent completing this checklist.

## Documentation (enhancements only)

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: `kind: refactor` — regression restore of an existing feature, no new user-facing surface)
- [x] ~~User-facing documentation updated~~ (N/A: refactor)
- [x] ~~Docs-checklist marked as done~~ (N/A: refactor)

**Docs Checklist:** N/A — refactor, section skipped per template instruction ("Skip this section for bugs and internal refactors").

## Final Checks

- [x] Commit message explains the why, not just what — PR commits explain the regression (feature orphaned in SPA migration #230) and the fail-closed operator fix.
- [x] No TODOs or FIXMEs left unaddressed — grep of changed core files clean.
- [x] Ready for another developer to use.

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI — PR #1065 open.
- [x] All CI checks pass — all code checks (Test, Lint, E2E, Frontend, Postgres, Cross-Compile, Fuzz) green; the `Rela Tickets` gate clears with this commit (ticket → done, this checklist → done).
- [x] PR URL documented below.

**PR:** https://github.com/sourcehaven-bv/rela/pull/1065
