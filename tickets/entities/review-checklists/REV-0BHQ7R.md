---
id: REV-0BHQ7R
type: review-checklist
title: 'Review: Extract Tier-B capability bindings off docRuntime (36 → 31)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`go test ./internal/...` green; CI green on every job except the Rela Tickets gate these files resolve)
- [x] Linters pass (golangci-lint 0 issues, arch-lint OK, plimsoll exit 0)
- [x] Coverage floors hold

## Code Review

- [x] ~~`/code-review` run~~ (N/A: pure receiver-only move, independently verified by the reviewer below rather than by the agent that wrote it)
- [x] All critical/significant findings addressed (none)
- [x] Diff independently inspected before commit: method count, both build tags, full suite, arch-lint and plimsoll all re-run by a second party

## Verification

- [x] Method count independently verified: docRuntime at 28 (after rebase onto the merged seed PR)
- [x] Fail-loud messages for nil capturer / nil apiClient verified byte-identical
- [x] `seed func() []SeedOp` verified to read live ops — registration runs once before any island while ops accumulate during execution, so a captured slice would always be empty
- [x] `luaFailer` confirmed in use at assert.go (consumer-side interface, not dead code)
- [x] `module.go` churn held to 4 lines to avoid conflicting with the parallel seed PR
- [x] ~~End-to-end screenshot replay exercised~~ (N/A: needs Chrome + a built SPA; ApplySeed/SeedOp unchanged and the moved bodies are byte-identical apart from the receiver — argued, not observed)
