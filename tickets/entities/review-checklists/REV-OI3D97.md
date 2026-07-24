---
id: REV-OI3D97
type: review-checklist
title: 'Review: Extract dataentry write nucleus into writeHandler'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`go test -race ./...` — full suite)
- [x] Lint clean (`golangci-lint run ./internal/dataentry/...`) — 0 issues
- [x] `gofmt` clean
- [x] `just plimsoll` passes — `App` directive ratcheted 132 → 115
- [x] `just arch-lint` passes
- [x] `just coverage-check` passes (76.0% total, all floors satisfied)
- [x] Builds across default / `postgres` / `memorybackend` tags

## Manual Review

- [x] `/code-review` run (cranky-code-reviewer over the full `develop...HEAD` diff) — **zero findings**. The reviewer re-derived every moved body by applying the substitution table to the develop originals and diffed line-for-line.
- [x] Snapshot-load parity verified — `a.Meta()` → `h.schema().Meta` preserves the exact per-handler load count (capture-once rule neither improved nor worsened; pure move)
- [x] Store aliasing verified — `h.store`/`h.manager`/`h.reader`/`currentEdgesByPeer` all resolve to the same store instance in both `NewApp` and `rebindApp`
- [x] Swappable collaborators (acl/audit) are live closures — ACL tests that reassign `app.acl` post-construction still take effect; `authorizeConflictResolve` re-authorizes via the same live instance as the write path
- [x] `writeMu` single-instance invariant — one `sync.Mutex` on App, pointer-shared to writeHandler/attachmentHandler/syncHandler, still locked directly by residual `actions.go`/`webhook.go`; 8 lock sites = 8 write endpoints, dry-run correctly lock-free, conflict-resolve lock scope identical to develop
- [x] Uniform-404 / affordance-denial-audit / ETag invariants preserved via shared closures (`gateRead`, `denyAfford`, `computeETag`); `lint_test.go` `acl.WriteRequest{Op:` rule holds
- [x] Wiring order verified — `app.write` built after `affordances`/`serializer`/`reader`/`entityManager`/`paths` in both `NewApp` and `rebindApp`; no stale-copy hazard
- [x] No leftovers — no orphaned definitions, stale `[App.*]` doc refs fixed, imports pruned

**Review Responses:** none — review returned zero findings
(critical/significant/minor/nit all empty).
