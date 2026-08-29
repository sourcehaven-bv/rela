---
id: REV-FK6U62
type: review-checklist
title: 'Review: Extract dataentry query/search leaf off App (92 → ~87), de-risking the read-pipeline steps'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`-count=1` and `-race` on dataentry, verified in the correct worktree with branch identity echoed alongside)
- [x] Linters pass (golangci-lint 0 issues, plimsoll at 86, arch-lint, comment-lint clean across 11081 comments, go vet)
- [x] Coverage floors hold (78.2%)

## Code Review

- [x] `/code-review` run (cranky-code-reviewer, diffed against the STACKED base tkt-8aj1pm-dataentry-appearance)
- [x] All critical/significant findings addressed: [[RR-LJHL3F]] — the reviewer's "plimsoll is inert on App" claim was investigated and **refuted**; the gate fires correctly (verified twice, exit status 3 in both the main checkout and the PR worktree)
- [x] Minor/nit findings addressed: [[RR-RYTNDG]] (overstated no-store claim precised; doubled godoc lead merged)

## Verification

- [x] **ACL invariant intact and NOT vacuous.** The seam is `search.VisibleSearcher` throughout; no path to a plain `search.Searcher`; `readGateFromContext` consulted identically with the deny-all short-circuit still ordered before any backend work. `TestACLSearch_DenyAllShortCircuit` carries a positive control asserting the backend runs exactly once for a granted search, so it cannot pass by never reaching the searcher. All 10 `TestACLSearch_*` pass under `-race`
- [x] **The closure deviation was proven empirically, not argued.** The reviewer implemented the rejected by-value design and ran the suite: four ACL tests fail, most damningly `TestACLSearch_ScopeErrorMapping` returning 200 with entity data where a 500 ACL failure was expected. A by-value capture keeps the construction-time searcher, so the injected failing searcher is never consulted and the test passes vacuously. The closures are load-bearing
- [x] `isRelationLinked` genuinely dead: only references were its definition and its own test; unexported, so no external dispatch; no reflection in the package. Deleted with its test
- [x] All five moved methods are pure moves — only substantive edits are a forced local rename (`q` → `sQuery`, since `q` now names the receiver) and one signature line-wrap
- [x] All call sites re-pointed (api_v1.go:326,1536; nextaction.go:66,107; scope.go:110); no survivors, none double-wrapped
- [x] Directive exact: 86 declared, 86 actual; the 92→86 delta reconciles precisely (5 moved + 1 deleted)
- [x] Zero lint suppressions added; test changes are mechanical re-points plus the one deletion, no assertion weakened
