---
id: REV-U67DAY
type: review-checklist
title: 'Review: mcp.Server round 2: lua/schema/resources/prompts handlers (38 → ~25)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (verified at the branch tip with `-count=1` in the correct worktree; `-race` on internal/mcp)
- [x] Linters pass (golangci-lint 0 issues, plimsoll at 25, arch-lint, comment-lint clean across 11075 comments)
- [x] Coverage floors hold

## Code Review

- [x] `/code-review` run (cranky-code-reviewer, diffed against the STACKED base tkt-yuetl7-mcp-trace-export, not develop)
- [x] All critical/significant findings addressed (none)
- [x] Minor/nit findings addressed: [[RR-8TF28O]] (handlerSet godoc now documents the anti-partial-wiring property, not just the gocritic reason), [[RR-046BDE]] (tracer package shadow removed). Third nit — `schemaResourceHandler`'s portmanteau name — deliberately not actioned; the shared dep set is real and splitting would yield two identical structs.

## Verification

- [x] `get_metamodel` / `get_schema` aliasing preserved — and now defended by dispatch_test.go's tool inventory ("deprecated alias, must keep dispatching"), so a future dedup fails a test rather than only contradicting a comment
- [x] Identity/ACL: principalMiddleware untouched at SDK level; no principal field on any handler; all three types hold the narrow GraphReader, never store.Store. resources/prompts narrowed their *reach* (same gate — Deps.Store was already GraphReader); TestACL_Resources_AreGated passes
- [x] TestLuaToolsHoldNoAmbientCapabilities remains load-bearing after the move (luaHandler.writeDeps is copied from the exact field it guards) — not orphaned by the extraction
- [x] handlerSet embedding verified: zero method promotion (so the plimsoll reduction is real, not accounting), no field-name collisions with Server's own four fields, and all four construction sites use a single whole-struct assignment — partial wiring is genuinely closed
- [x] NewServer validates deps BEFORE calling handlers(), so groups are never built from unvalidated nils
- [x] Method count verified at 25, matching the directive
- [x] Tests: receiver re-points only; zero assertion changes, zero deleted tests, zero golden-file changes (capabilities_posture_test.go and golden_test.go byte-identical to base)
