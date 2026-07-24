---
id: IMPL-WC2O2G
type: implementation-checklist
title: 'Implementation: Ship a generated operator handbook for the demo tracker (dogfood rela-docs)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Manual authored + built to `docs/examples/ticket-tracker-manual.md` (+ `ticket-form.png`), committed.
- [x] `just docs-example` target added (kept out of `docs`/`docs-check` — screenshot PNG not byte-reproducible).
- [x] README links the handbook (via `generate-docs.lua`; README regenerated).
- [x] Renderer fix: island-seam + echo trailing-newline trimming (`internal/docs/runtime.go`), fixing MD012 double-blanks.
- [x] Error handling: fix preserves literal (fenced) content verbatim — no over-collapsing.

## Test Quality

- [x] Table-driven where useful; behavior-focused assertions.
- [x] `TestBuild_NoDoubleBlankAtSeam`, `TestBuild_PreservesBlanksInsideLiteralFence`, `TestBuild_EchoTrailingNewlineTrimmed`, `TestBuild_SingleTrailingNewline`.

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified

**Verification Evidence:**
- `just docs-example` renders the handbook: field table, mermaid lifecycle (Start work/Mark resolved/Close/Reopen labels), per-status meanings, relations list, schema graph, seeded worked example, editor/viewer roles matrix, annotated screenshot (clip=focus on status+priority). ✓
- `markdownlint` on the handbook + README → 0 issues. ✓
- Regression proof: a `​```markdown` sample with intentional double blanks is preserved verbatim (was wrongly collapsed by the first regex fix; now correct). ✓
- `go test ./internal/docs/` green; coverage 77.7%; `just coverage-check` PASS (total 76.4%).

## Quality

- [x] Follows project patterns (seam-scoped trim, not a global regex over rendered output)
- [x] DRY: no premature abstraction
- [x] No security issues (ephemeral seeded store; no real data)
- [x] No silent failures
- [x] No debug code left behind

**Gates:** golangci-lint 0 issues · `go vet` clean · coverage-check PASS.
