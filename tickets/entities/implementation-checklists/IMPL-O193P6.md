---
id: IMPL-O193P6
type: implementation-checklist
title: 'Implementation: Per-command ACL guard: gate command execution and button visibility on a named permission (entity/list/global; view deferred)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Write tests first (TDD where applicable)
- [x] Implement the happy path
- [x] Handle edge cases identified in planning
- [x] Add error handling

**TDD, genuinely:** `TestCommandExecReadOnlyDenied` was written and run *before*
any implementation, and it **reproduced the live vulnerability** — all four
contexts returned 200 and executed the script under `ReadOnlyACL`:

```
--- FAIL: TestCommandExecReadOnlyDenied/entity
    read-only must deny command exec: expected 403, got 200: event: message
    data: {"type":"message","text":"ran"}
```

RR-CWWJGW was therefore demonstrated, not merely asserted, and the same test now
passes.

## Quality

- [x] Code follows project style
- [x] No security issues introduced
- [x] Performance is acceptable
- [x] No debug code left behind

## What was built

**`internal/dataentryconfig/config.go`** — `CommandConfig.Permission`
(`permission:` YAML key) with godoc covering the bimodal policy and the view
carve-out.

**`internal/dataentry/commands.go`** — `authorizeCommand(ctx, acl, cmd)`, the
single decision point, plus `commandDenyReason`. Called by *both*
`handleCommandExec` (the boundary, 403) and `resolveCommands` (the UI filter),
so the rendered button set cannot drift from what exec allows.

**`internal/dataentry/command_handler.go`** — `aclImpl func() acl.ACL` field +
`currentACL()` accessor.

**`internal/dataentry/app.go`**, **`test_helpers_test.go`** — wire the closure
at both construction sites.

**`internal/dataentryconfig/validate.go`** — `viewCommandPermissionWarnings` via
the existing `CollectConfigWarnings` channel (a non-fatal channel already
existed; no new mechanism invented).

**Docs** — `docs/data-entry.md` (Authorization section + `permission:` row),
`docs/acl-security.md` (`command:*` family alongside `history:read`),
`e2e/tests/read-only-mode.spec.ts` (corrected the false claim).

## Two hardening decisions beyond the plan

1. **Nil-ACL denies.** `authorizeCommand` returns false on a nil ACL, and
`currentACL()` returns nil rather than panicking on an unwired closure. The
first draft panicked (nil deref at `commands.go:362`) when `newHandlerTestApp`
built a handler without the field. A guard that panics on a wiring bug is bad;
one that *grants* would be worse — it would look present in review. Pinned by
`TestAuthorizeCommandNilACLDenies`.

2. **Explicit default-arm comment.** The `default:` (fail-open) arm carries a
note that it exists *only* because it is the no-policy case, and that a future
restrictive ACL must get its own arm rather than inheriting it.

## Verification — every gate run, all green

| Gate | Result |
|---|---|
| `go build ./...` | pass |
| `go test ./...` (whole repo) | pass |
| `just lint` | **0 issues** (4 self-inflicted issues found and fixed properly, not suppressed) |
| `just arch-lint` | OK — no warnings |
| `just coverage-check` | package floor + total both PASS; total 76.3% |
| `just plimsoll` | pass (logic on `commandHandler`, not `App`) |

Lint fixes were real, not `//nolint`: split a multi-line `if` into a guard
clause, moved `ctx` to the first parameter position (revive), and replaced a
`context.Context` struct field with a `user string` + per-case
`principalCtx(...)` (containedctx).

**Pre-existing and unrelated:** `e2e` `tsc` reports two config errors (`Cannot
find type definition file for 'node'`, removed `moduleResolution=node10`).
Verified identical with my file stashed — I only edited a comment there.

## Test coverage added

- `TestCommandExecReadOnlyDenied` — 4 contexts; the canary
- `TestCommandExecNopACLFailsOpen` — 4 contexts; fail-open regression
- `TestCommandExecDeclarativeFailsClosed` — 10 cases incl. **view denied with a
set-and-granted permission**
- `TestResolveCommandsFiltersUnauthorized` — 4 cases across all three ACL modes
- `TestAuthorizeCommandNilACLDenies` — wiring-bug guard
- `TestViewCommandPermissionWarning` — 3 cases

All 9 ticket acceptance criteria are covered. The 403 body is asserted not to
echo the permission name.
