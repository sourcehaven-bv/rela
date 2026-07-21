---
id: PLAN-CJ49VK
type: planning-checklist
title: 'Planning: Per-command ACL guard: gate command execution and button visibility on a named permission (entity/list/global; view deferred)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:** see TKT-MJ02AO. Fine-grained `permission:` for `entity`/`list`/
`global`; `view` denied outright under a configured policy; blanket deny under
read-only. Fail policy per **DEC-EIHQSU**.

## Research

- [x] ~~run `/research`~~ (N/A: DEC-EIHQSU settles the policy; the named-permission pattern already exists in-tree)
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A

**Existing Solutions — in-tree prior art, reused not reinvented:**

- `acl.PermHistoryRead` (`internal/acl/policy.go:43`) — the named-permission
precedent: a global capability with no entity subject, granted via a role's
`permissions:` list.
- `authorizeHistoryRead` (`internal/dataentry/history_handler.go:100-127`) —
the call shape: `gate := readGateFromContext(ctx)` then
`gate.HoldsPermission(ctx, perm)`.
- `router.go:125` (`if d, ok := a.acl.(*acl.Declarative); ok`) and
`watcher.go:472` — the established **type-assertion** idiom for detecting ACL
mode. This is how mode detection is done here; do not invent a new one.

## CRITICAL FINDING — the read gate alone cannot implement DEC-EIHQSU

`readGateFromContext` returns `nopReadGate` for **both** `NopACL` and
`ReadOnlyACL` (`readgate.go` doc: "the gate the handlers see under NopACL /
ReadOnlyACL"), and:

```go
// readgate.go:106-108
func (nopReadGate) HoldsPermission(context.Context, string) bool { return true }
```

`attachACLRequest` only opens an `acl.Request` when the ACL is
`*acl.Declarative` (`router.go:125`, and its doc: "NopACL / ReadOnlyACL paths
don't open Requests").

**Consequence:** a naive `if !gate.HoldsPermission(ctx, perm) { 403 }` would
return `true` under read-only and **fail open** — reproducing RR-CWWJGW, the
exact bug this ticket exists to close. The gate distinguishes *Declarative vs
not*; it cannot distinguish *NopACL vs ReadOnlyACL*, and DEC-EIHQSU needs all
three modes.

**Resolution:** discriminate on `App.acl` by type assertion (the in-tree idiom),
then consult the gate only in the Declarative branch:

| `App.acl` type | behavior |
|---|---|
| `*acl.Declarative` | fail-closed; `permission:` required and checked via `gate.HoldsPermission`; `view` denied unconditionally |
| `acl.ReadOnlyACL` | deny **all** command exec, all contexts |
| anything else (`NopACL`) | fail-open; execute as today |

Ordering matters: check ReadOnlyACL **before** the Declarative branch, and
default-deny only in the Declarative branch — a future third ACL impl must not
silently land in the fail-open default. Add a comment saying so.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

1. **`CommandConfig.Permission`** — new `permission` YAML key
(`internal/dataentryconfig/config.go:563`).

2. **`commandHandler.aclMode`** — a new narrow closure field on the handler
(`command_handler.go:26-31`), consistent with its existing consumer-side closure
style (`schema`, `services`, `projectRoot`, `executeView`). Returns a small enum
resolved from `App.acl`, so the handler never imports the ACL decision logic
itself.

3. **`authorizeCommand(ctx, cmd) (bool, string)`** — one function, the single
decision point, returning allow + a deny reason for the 403 body. Used by
**both** exec and resolve so they cannot drift.

4. **Exec enforcement** — call it in `handleCommandExec` before the context
switch (`commands.go:301`), 403 on deny.

5. **Resolve filtering** — call it in `resolveCommands` (`commands.go:47`) so
`GET /_commands` omits unexecutable commands.

6. **`validateCommands` warning** — `permission:` set on a `context: view`
command warns (the key is not honored; silently ignoring would mislead).

**Alternatives considered:**

- *`gate.HoldsPermission` alone.* **Rejected — fails open under read-only**
(see Critical Finding). This is the trap this plan exists to document.
- *Extend `acl.ACL` with an `AuthorizeCommand` method.* Rejected: changes a
sealed, widely-implemented interface for one consumer; `Subject` is a sealed sum
and a command has no entity subject.
- *Enforce only at exec, no resolve filtering.* Rejected: buttons would render
and then 403 on click — the UX TKT-72SCPR depends on requires the filter.
- *Separate allow-logic for exec and resolve.* Rejected: two paths that must
agree will drift. One function, two callers.

**Files to modify:**

- `internal/dataentryconfig/config.go` — `Permission` field
- `internal/dataentryconfig/validate.go` — view+permission warning
- `internal/dataentry/command_handler.go` — `aclMode` closure
- `internal/dataentry/commands.go` — `authorizeCommand`, exec 403, resolve filter
- `internal/dataentry/app.go` — wire `aclMode` (~L588, next to the existing `acl:` closure)
- `internal/dataentry/commands_test.go`, `internal/dataentryconfig/validate_test.go` — tests
- `docs/data-entry.md`, `docs/acl-security.md`
- `e2e/tests/read-only-mode.spec.ts` — correct the deferred-sites comment

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input sources:** `permission:` is admin-authored config, validated at load.
The request supplies only the command ID (looked up in a closed map) and the
context IDs — unchanged by this ticket.

**Security-sensitive operation:** this *is* the authorization layer for a `sh
-c` endpoint. Two failure modes to guard:

1. **Fail-open under read-only** — the Critical Finding above. Pinned by a test
asserting 403 under `ReadOnlyACL` *with* a granted permission.
2. **Default-deny placement** — the deny must be the default *inside* the
Declarative branch, not a fallthrough. A future ACL impl must not inherit
fail-open by accident.

**Error handling:** the 403 body names the denial in general terms and must not
echo the required permission name or policy contents — consistent with
`acl.Decision.Reason` ("never contains raw policy data so 403 bodies don't leak
the full effective-role set", `acl.go:110-112`). Do not include the
`permission:` value in the response.

**Not weakened:** POST-only (`commands.go:282`, the `<img src>` RCE fix),
same-origin/local-host middleware, and `cmd.label` text interpolation all
unchanged.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

All Go, via `httptest`, alongside the existing `commands_test.go` suite. Per
`e2e/tests/AGENTS.md:45` these are HTTP-shape assertions and belong in Go, not
Playwright.

| Scenario | Expect |
|---|---|
| Declarative, `permission: X`, principal holds X | 200, command runs |
| Declarative, `permission: X`, principal lacks X | 403 |
| Declarative, no `permission:`, entity/list/global | 403 (fail-closed) |
| Declarative, `context: view`, `permission:` set **and granted** | 403 (key not honored) |
| Declarative, `context: view`, no permission | 403 |
| **ReadOnlyACL, `permission:` set and granted** | **403 — the fail-open trap** |
| ReadOnlyACL, any context incl. view | 403 |
| NopACL, no `permission:`, all four contexts incl. view | 200 (fail-open regression) |
| NopACL, `permission:` set | 200 (no policy to check against) |
| `resolveCommands` under Declarative | denied commands absent from the list |
| `resolveCommands` under NopACL | list unchanged from today |
| `validateCommands`, `permission:` on view command | warning emitted |

**Edge cases:**

- Empty-string `permission:` — treat as unset, not as a permission named `""`
- Command ID present but principal denied — 403, not 404 (the command's
existence is already public via config; no oracle concern)
- Resolve returning an empty list — must serialize as `[]`, not `null`
(frontend iterates it)
- View command under NopACL must still populate `RELA_VIEW_ID` etc. — the
deferral must not alter the payload

**Negative tests:** malformed/unknown permission name in config → validation
error or warning per the decision; 403 body must not contain the permission
name.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

1. **Silent fail-open** — the Critical Finding. *Mitigation:* the ReadOnlyACL +
granted-permission test is the canary; write it first.
2. **Breaking existing deployments** — DEC-EIHQSU accepts this for
`acl.yaml` users. *Mitigation:* NopACL regression test across all four contexts;
release note.
3. **Exec/resolve drift** — *Mitigation:* one `authorizeCommand`, two callers.
4. **Test-helper reassignment** — `test_helpers_test.go:149` does
`app.acl = svc.ACL()` after construction, and `app.go:494/588` deliberately use
closures (`func() acl.ACL { return app.acl }`) to tolerate that. The new
`aclMode` closure **must** follow the same late-binding pattern or tests that
swap the ACL post-construction will see a stale mode.
5. **`plimsoll` god-object limits** — adding methods to `App`. *Mitigation:*
put the logic on `commandHandler`/free functions, not `App`.

**Effort:** `l` — confirmed.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

- [x] `docs/data-entry.md` — the `permission:` key; that `view` cannot be gated per-command yet; that commands are denied under `--read-only`
- [x] `docs/acl-security.md` — the command-permission family alongside `history:read`; the bimodal fail policy (DEC-EIHQSU)
- [x] `e2e/tests/read-only-mode.spec.ts:8` — correct the false "403 at the server on click" claim for command buttons
- [ ] ~~docs/metamodel.md~~ (N/A: no metamodel change)
- [ ] ~~docs/cli-reference.md~~ (N/A: no CLI change)
- [ ] ~~README.md~~ (N/A: not project-level)

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** this ticket *is* the remediation of RR-65KG68,
RR-CWWJGW and RR-L6UXCF from TKT-72SCPR's review. The Critical Finding above was
surfaced during this ticket's own planning by reading `readgate.go` before
writing code — it would have produced a silently fail-open guard.

Policy settled in **DEC-EIHQSU** (accepted 2026-07-20).
