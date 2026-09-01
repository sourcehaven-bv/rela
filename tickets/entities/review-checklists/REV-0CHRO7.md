---
id: REV-0CHRO7
type: review-checklist
title: 'Review: Extract the seed cluster off docRuntime (36 → 33)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`go test ./internal/...` green; CI green on every job except the Rela Tickets gate these files resolve)
- [x] Linters pass (golangci-lint 0 issues, arch-lint OK, comment-lint clean, plimsoll exit 0)
- [x] Coverage floors hold

## Code Review

- [x] ~~`/code-review` run~~ (N/A: pure receiver-only move; the design decision below was independently confirmed)
- [x] All critical/significant findings addressed (none)
- [x] Embedding-vs-named-field decision reviewed and endorsed: plimsoll v0.2.0 counts only directly-declared methods, so embedding would have reported 33 while leaving every seed method callable on dr — satisfying the linter without severing the reach it exists to detect

## Verification

- [x] Method count independently verified: docRuntime at 33
- [x] Minted id values and sequence unchanged (method bodies byte-identical apart from receiver and field renames)
- [x] `SeedOp` / `ApplySeed` shape and signature unchanged, so internal/docscapture is untouched
- [x] Cross-boundary edits held to three one-token `dr.seed.ops` reads in the parallel PR's files, no reordering or reformatting
- [x] ~~End-to-end screenshot replay exercised~~ (N/A: needs Chrome + a built SPA; internal/docscapture passed from cache, legitimate only because ApplySeed/SeedOp are unchanged — argued, not observed)
