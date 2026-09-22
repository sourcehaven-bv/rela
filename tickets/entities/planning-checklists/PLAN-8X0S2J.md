---
id: PLAN-8X0S2J
type: planning-checklist
title: 'Planning: Command exec ungated under default NopACL: refuse on non-loopback bind, with an explicit override flag'
status: done
---

<!-- @managed: claude-workflow v1 -->

## User decisions (confirmed)

- **Injection shape:** authorizer is a **trailing positional param on `NewApp`**
(12th arg). Update every call site; add a defaulting test helper (→ `ungated`)
so test call sites don't each need editing.
- **Rollout:** **hard behavior change** — non-loopback + no `acl.yaml` + no
override refuses command exec immediately (was: ran). Restore path is
`--allow-unauthenticated-commands` OR adding an `acl.yaml`. Ship with a loud
startup warning + prominent PR/changelog/docs note. NOT a warn-only transition
release (a security gap should not stay open a cycle).

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

IN:
- Replace the `authorizeCommand` type-switch with a `commandAuthorizer` interface,
  with `ungated`/`gated`/`deny` implementations selected once at the wiring site.
- Refuse ungated command exec on a **non-loopback** bind under NopACL (the fix).
- `--allow-unauthenticated-commands` flag (+ `RELA_ALLOW_UNAUTHENTICATED_COMMANDS=1`)
as the deliberate opt-out, modeled on `--unconfined-commands`.
- Loud startup log when the override is active.
- Docs in `docs/server-security.md`.

OUT:
- The `context: view` fine-grained permission (still deferred, unchanged — the
gated impl keeps denying view commands).
- The remote file-delivery problem (TKT-PYPNWO).
- Any change to Declarative/ReadOnly semantics — those arms behave exactly as today.
- Adding an ACL check to `/api/open-file` (separate surface; note in PYPNWO).

**Wiring finding that shapes the change (verified):** `commandHandler.aclImpl`
is a closure over `app.acl` (`app.go:664`), and **`App` never learns the bind
address** — the bind (`f.bind`) lives only in `cmd/rela-server`. So unlike the
read-gate precedent (`scriptEntityReader` decides purely from `d == nil`,
available inside `appbuild`), the command authorizer's choice depends on
(policy, bind, override) — two of which are only known in `cmd/rela-server`.
Therefore the authorizer is **constructed in the cmd layer and injected into
`App`** (trailing `NewApp` param, per the decision above).
`shouldWarnNoACL(svc.ACL(), f.readOnly)` at `main.go:450,540` already computes
the "NopACL and not read-only" predicate right where the bind is in scope — the
authorizer decision belongs next to it.

**Acceptance Criteria:**

1. Loopback bind (`127.0.0.1`) + no `acl.yaml` → command runs (desktop/dev
unchanged). POST `/api/command/<id>` returns the SSE stream.
2. Non-loopback bind (`0.0.0.0`) + no `acl.yaml` + no override → command exec
**403s**, and `resolveCommands` omits the command from the UI listing.
3. Non-loopback + no `acl.yaml` + `--allow-unauthenticated-commands` → command
runs, and a loud warning is logged once at startup.
4. Declarative policy present → unchanged: `cmd.Permission` held ⇒ run, unheld ⇒
403, `context: view` ⇒ 403. Bind and override are irrelevant.
5. `--read-only` (`ReadOnlyACL`) → 403 regardless of bind/override
(`TestCommandExecReadOnlyDenied` still passes).
6. `rela-desktop` (Wails, no HTTP listener) → command runs (treated as
loopback-equivalent).
7. The `commandHandler` contains no `acl.ACL` type-switch after the change (grep
assertion); the decision is one wiring line.

## Research

- [x] ~~Run `/research`~~ (N/A: design settled with the user; direct in-tree
precedent exists)
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations

**Research Doc:** N/A.

**Existing Solutions (the two precedents this copies):**
- **Two-impl-at-the-seam:** `scriptEntityReader`/`scriptTracer`
(`internal/appbuild/appbuild.go`) return raw / `PolicyReader` / `DenyReader`
based on the wired ACL; the consumer holds an interface (`lua.EntityReader`,
`tracer.Tracer`) and never type-switches. DEC-ZBI39P. `DenyReader`
(`internal/visibility/denyreader.go`) is the "refuse, don't fail open" model for
the deny impl.
- **Refuse-by-default-with-named-override:** `--unconfined-commands` /
`RELA_UNCONFINED_COMMANDS=1` (`main.go:91`) — same "platform refuses a risky
thing; a host isolated at another layer opts back in explicitly" shape, for the
sandbox dimension of the *same* shell-exec surface. Copy its flag/env/help
pattern verbatim in spirit.
- **`context: view` deny + the fail-closed switch** already live in
`authorizeCommand` (`commands.go:84-119`); the gated impl is that body, moved
behind the interface — not new logic.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified

**Technical Approach (mechanical):**

1. **`internal/dataentry/commands.go`** — introduce
   ```go
   type commandAuthorizer interface { Authorize(ctx context.Context, cmd CommandConfig) bool }
   ```
with three impls:
- `ungatedAuthorizer struct{}` → `true` (named for greppability, cf. TKT-1WV50C).
- `denyAuthorizer struct{}` → `false`.
- `gatedAuthorizer struct{ d *acl.Declarative }` → the current `*acl.Declarative`
arm body: `context: view` ⇒ false; empty `Permission` ⇒ false; else
`readGateFromContext(ctx).HoldsPermission(ctx, cmd.Permission)`.
**LOAD-BEARING (RR-QWVG8Y):** constructed ONLY from a non-nil `*acl.Declarative`
(constructor rejects nil). NEVER the fallback for NopACL/ReadOnly/unknown — because
`readGateFromContext` returns the permissive `nopReadGate` under BOTH NopACL and
ReadOnly, so a gated impl on those paths would fail OPEN (the RR-CWWJGW bug). The
wiring seam is the sole distinguisher; the ctx read gate cannot self-distinguish
NopACL vs ReadOnly.
Delete the old `authorizeCommand` type-switch.
**INVARIANT (RR-2TVXO7):** both `resolveCommands` and `handleCommandExec` call the
SAME `h.authz.Authorize(ctx, cmd)` — one field, one instance, never re-derived per
site (keeps DEC-EIHQSU's single-decision-point property; the button filter and the
403 boundary can't drift).

2. **`internal/dataentry/command_handler.go`** — replace `aclImpl func() acl.ACL`
with `authz commandAuthorizer`; drop `currentACL()`. Value, not closure — audit
the late-binding tests and inject the authorizer they need.

3. **`internal/dataentry/app.go`** — `NewApp` takes `commandAuthorizer` as the
**trailing (12th) positional param**. Wire into `app.commands`. `aclImpl == nil`
guard stays for the ACL; a nil authorizer is treated as `denyAuthorizer`
(fail-closed). Add a test helper (e.g. `newTestApp`/defaulting constructor) that
passes `ungatedAuthorizer{}` so existing test call sites need no per-site edit.

4. **`cmd/rela-server/main.go`** — new flag beside `--unconfined-commands`:
   ```go
   flag.BoolVar(&f.allowUnauthCommands, "allow-unauthenticated-commands",
     os.Getenv("RELA_ALLOW_UNAUTHENTICATED_COMMANDS") == "1", "…")
   ```
Add `selectCommandAuthorizer(active acl.ACL, bind string, override bool)
commandAuthorizer` decided next to `shouldWarnNoACL`:
- `ReadOnlyACL` → deny
- `*acl.Declarative` → gated
- `NopACL` + loopback → ungated
- `NopACL` + non-loopback + override → ungated (+ loud `slog.Warn`)
- `NopACL` + non-loopback + no override → deny
Reuse `isLoopbackHost(host)` (`main.go:505`). Pass the result as the trailing
`NewApp` arg.

5. **Desktop** (`cmd/rela-desktop`) — passes `ungatedAuthorizer{}` (no bind ⇒
loopback-equivalent). Update the desktop NewApp call site.

6. **Docs** — `docs/server-security.md`: a subsection next to `--unconfined-commands`
   and the non-loopback warning, stating the network default and the override,
   with a prominent upgrade note (hard change).

**Alternatives considered:**
- *Decide the authorizer inside `appbuild`/`dataentry` from the ACL alone* —
rejected: the bind isn't known there.
- *Keep the type-switch, add a bind branch* — rejected by the user.
- *A generic `acl.CommandACL` in the `acl` package* — rejected: `acl` doesn't
depend on bind/deployment concepts.
- *Post-construction setter / options struct for injection* — rejected by the user
in favor of the trailing param.

**Files to modify:** `internal/dataentry/commands.go`,
`internal/dataentry/command_handler.go`, `internal/dataentry/app.go`,
`cmd/rela-server/main.go`, `cmd/rela-desktop/main.go`,
**`internal/docscapture/server.go` (RR-8HJYDL — third `NewApp` call site @:97;
passes `ungatedAuthorizer{}`, loopback-equivalent)**,
`docs/server-security.md`, plus tests (below).

**Fail-closed at the boundary (RR-8HJYDL):** `NewApp` rejects a nil authorizer
(treat as deny), so a forgotten call site fails the build or fails closed+loud —
never open.

## Security Considerations

- [x] Input sources identified
- [x] Input validation approach defined (allowlist / fail-closed)
- [x] Security-sensitive operations identified
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**
- Bind address: operator-supplied `--bind`, already parsed; `isLoopbackHost` is the
existing allowlist (localhost/127.0.0.1/::1).
- Override: operator-supplied flag/env only — never a request input.
- No new request-derived data influences the authorization decision.

**Security-Sensitive Operations:**
- Authorization decision for arbitrary `sh -c`. **Fail-closed by construction**:
network default is `deny`; unknown/nil authorizer is `deny`; only paths to
`ungated` are (loopback) or (explicit override).
- 403 body stays coarse (`commandDenyReason`) — no bind/policy/permission detail.
- The override warning is operator-facing (log), never in an HTTP response.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined
- [x] Integration test approach defined

**Test Scenarios (mapped to AC):**
- AC1/6 — `selectCommandAuthorizer(NopACL, "127.0.0.1", false)` → ungated; +
desktop-equivalent (empty/loopback host).
- AC2 — `selectCommandAuthorizer(NopACL, "0.0.0.0", false)` → deny; **and**
`handleCommandExec` with deny authorizer → 403; `resolveCommands` omits it.
- AC3 — `selectCommandAuthorizer(NopACL, "0.0.0.0", true)` → ungated; assert the
warning is emitted (capture slog).
- AC4 — Declarative: permission-held → allowed, unheld → 403, `context:view` → 403.
  Port the existing Declarative-arm tests to `gatedAuthorizer`.
- AC5 — preserve `TestCommandExecReadOnlyDenied` (now via `denyAuthorizer`).
  **RR-QWVG8Y:** keep the assertion that ReadOnly 403s EVEN THOUGH its ctx read
  gate answers permissive — that's the whole point of the canary.
- **Test migration (RR-CWBZVT):** the `app.acl =` late-binding tests
  (`commands_test.go:1061,1102,1197,1229,1258,1267`) must set
  `app.commands.authz = …` directly (deny/gated/ungated) instead of reassigning
  `app.acl` — otherwise they'd exercise the wrong authorizer and the ReadOnly canary
  would pass vacuously. Add a grep guard flagging `app.acl =` in command tests.
- AC7 — grep/AST assertion: no `acl.ACL` type-switch in
`command_handler.go`/`commands.go`.

**Edge Cases:**
- `::1` / IPv6 loopback → ungated (isLoopbackHost covers it).
- Override **with** a Declarative policy → policy wins (gated). Test precedence so
the flag can't weaken a configured policy.
- Override on a **loopback** bind → no-op (already ungated); assert harmless.
- nil authorizer (wiring bug) → deny.

**Negative Tests:**
- Non-POST to `/api/command/` still 405.
- Deny path returns coarse 403, no permission name in the body.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed
- [x] Effort estimated

**Risks:**
- *Hard behavior change for non-loopback + no-acl deployments* — deliberate
(user-confirmed). Mitigation: loud startup warning + documented one-line restore
(`--allow-unauthenticated-commands` or `acl.yaml`) + prominent PR/changelog
note.
- *`NewApp` signature churn across call sites* — LOW/MEDIUM. Mitigation: defaulting
test helper (→ ungated); grep call sites first.
- *Late-binding tests reassigning `app.acl`* — audit and inject an authorizer
instead; keep a `func() commandAuthorizer` shim only if genuinely required.

**Effort:** m

## Documentation Planning

- [x] User-facing docs identified
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**
- [x] `docs/server-security.md` — network default + `--allow-unauthenticated-commands`,
next to `--unconfined-commands` + the non-loopback sections; hard-change upgrade
note
- [x] `docs-project/entities/guides/GUIDE-acl-security.md` — mirror if it documents
command gating (check during impl)
- [x] N/A metamodel.md / cli-reference.md

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:**

- **RR-QWVG8Y (critical, addressed)** — `gatedAuthorizer` must be constructed
  ONLY from a concrete non-nil `*acl.Declarative`, never as the fallback for an
  unknown/nop ACL. The ctx read gate (`readGateFromContext`) returns the
  permissive `nopReadGate` under BOTH NopACL and ReadOnlyACL, so a gated impl that
  consults only the ctx gate fails OPEN under ReadOnly — the exact RR-CWWJGW bug.
  The wiring seam is the distinguisher, by design. See amended Approach step 1 + AC5.
- **RR-CWBZVT (significant, addressed)** — the `app.acl =` late-binding tests
  (`commands_test.go:1061,1102,1197,1229,1258,1267`) must be migrated to inject the
  authorizer directly (`app.commands.authz = …`), else they'd test the wrong
  authorizer and the ReadOnly canary would pass vacuously. See amended Test Plan.
- **RR-8HJYDL (significant, addressed)** — third `NewApp` call site
  `internal/docscapture/server.go:97` added to the file list; passes
  `ungatedAuthorizer{}` (loopback-equivalent). nil authorizer at the boundary →
  deny (fail closed).
- **RR-2TVXO7 (minor, addressed)** — pin the invariant that `resolveCommands` and
  `handleCommandExec` call the SAME `h.authz.Authorize`, never re-derived per site
  (preserves DEC-EIHQSU's single-decision-point property).

All findings addressed in-plan below. No critical/significant findings remain open.
