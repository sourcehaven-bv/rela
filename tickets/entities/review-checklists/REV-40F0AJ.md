---
id: REV-40F0AJ
type: review-checklist
title: 'Review: Extract HTTP/cache/AI binding clusters off lua.Runtime (ratchet 60 → ~45)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (full suite + `-race ./internal/lua/` locally; CI green on all jobs except the Rela Tickets gate, which this ticket's files landing on the branch resolves)
- [x] Linters pass (golangci-lint, plimsoll with directive ratcheted 60 → 45, arch-lint, comment-lint)
- [x] Coverage floors hold

## Code Review

- [x] `/code-review` run (cranky-code-reviewer, independent verification of the 79dd186c..9084dfbc diff)
- [x] All critical/significant findings addressed (none — verdict: sound, genuine pure move)
- [x] Nit recorded: [[RR-ZECAQE]] (godoc dropped a sentence that was stale anyway — wont-fix with reason)

## Verification

- [x] Capability gates verified byte-identical (caps.AI / caps.HTTP blocks in registerBindings unchanged; only line numbers shifted)
- [x] scriptPath closure verified to read the live field — SetScriptPath and RunFileContent mutations both observed post-registration
- [x] aiProvider/cache by-value snapshots verified safe: only writers are Options, applied before registerBindings
- [x] Zero test-file changes; cache namespacing pinned by untouched cache_test.go
- [x] Method count independently verified: Runtime 45, matching the directive
