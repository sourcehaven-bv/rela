---
id: IMPL-XMYIIT
type: implementation-checklist
title: 'Implementation: Command exec ungated under default NopACL: refuse on non-loopback bind, with an explicit override flag'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code (`TestSelectCommandAuthorizer` table over
the full decision matrix; `TestCommandHandlerNilAuthorizerDenies`)
- [x] Integration tests written (the exec-path tests drive `handleCommandExec`
end-to-end through the authorizer: `TestCommandExecReadOnlyDenied`,
`TestCommandExecNopACLFailsOpen`, `TestCommandExecDeclarativeFailsClosed`,
`TestResolveCommandsFiltersUnauthorized`)
- [x] Happy path implemented (loopback/desktop → ungated; policy → gated)
- [x] Edge cases from planning handled (IPv6 loopback, override-with-policy
precedence, override-on-loopback no-op, nil authorizer → deny)
- [x] Error handling in place (`newGatedAuthorizer`/`SelectCommandAuthorizer`
return errors; `NewApp` rejects nil authorizer; cmd exits loudly on select
error)

## Test Quality

- [x] Using fixture builders (`newHandlerTestApp`, `commandPolicyACL`,
`mustGatedAuthorizer`, `mustNewACL`)
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter (authorizer *type* asserted via
`authorizerKind`, not deep-equal over the policy pointer)
- [x] ~~Interpolated values from objects~~ (N/A: assertions are on authorizer kind
  + HTTP status, no interpolation)
- [x] Property comparisons use the impl under test, not hardcoded strings

## Manual Verification

- [x] Feature verified via the automated exec-path + selector tests (they are the
end-to-end flow: real ACL, real read gate on ctx, real handler)
- [x] Each acceptance criterion verified
- [x] Edge cases verified

**Verification Evidence:**

Build + checks, all green on branch `fix/command-auth-network-refusal` (rebased
on `origin/develop` @ latest):

- `go build ./...` — clean; `go build -tags postgres/-tags memorybackend` — clean
(backend-import separation preserved).
- `go vet` — clean. `gofmt -l` — clean. `golangci-lint` — 0 issues.
- `just arch-lint` — OK, no warnings. `just plimsoll` — passes (NewApp gained a
param but no god-object line tripped).
- `just coverage-check` — PASS (package floor + total). dataentry at 79.5%.
- Round-trip corpus (`TestMdCorpusRoundTrip`) — passes for all new ticket/RR/plan md.

AC-by-AC (all via passing tests):
- AC1 loopback+no-policy → ungated → 200: `TestCommandExecNopACLFailsOpen` (all 4 contexts).
- AC2 network+no-policy+no-override → deny → 403 + button omitted:
`TestSelectCommandAuthorizer` "nop + network, no override → deny" +
`TestCommandExecReadOnlyDenied` (deny authorizer → 403) +
`TestResolveCommandsFiltersUnauthorized` "read-only hides every command".
- AC3 network+no-policy+override → ungated + warning fires:
`TestSelectCommandAuthorizer` "nop + network + override → ungated (fires)"
asserts `onOverride` invoked exactly on that path.
- AC4 Declarative held/unheld/view: `TestCommandExecDeclarativeFailsClosed`
(granted→200, not-held→403, no-permission→403, view→403 even when granted).
- AC5 ReadOnly → 403 despite permissive ctx gate: `TestCommandExecReadOnlyDenied`
(permission set AND would be granted; only the deny authorizer produces the
403).
- AC6 desktop (no bind) → ungated: `UngatedCommandAuthorizer()` wired in
`cmd/rela-desktop` + `internal/docscapture`; selector loopback case covers it.
- AC7 no acl.ACL type-switch in command source: `TestCommandAuthorizationHasNoACLTypeSwitch`.

Design-review findings, all verified closed in code:
- RR-QWVG8Y: `gatedAuthorizer` only built via `newGatedAuthorizer` (nil-rejecting);
grep confirms no bare `gatedAuthorizer{}` construction in production.
- RR-CWBZVT: command-auth tests set `app.commands.authz` directly;
`TestCommandTestsDriveAuthorizerNotACL` grep-guards against `app.acl =`
reappearing.
- RR-8HJYDL: `internal/docscapture/server.go` updated; `NewApp` rejects nil authz.
- RR-2TVXO7: single `h.authorizer()` used by both `resolveCommands` and
`handleCommandExec`.

## Quality

- [x] Code follows project patterns (mirrors `scriptEntityReader`/`DenyReader`
two-impl-at-seam; flag mirrors `--unconfined-commands`)
- [x] Checked for DRY opportunities — extracted `warnUnauthenticatedCommands`,
`mustGatedAuthorizer`, `authorizerKind`, `mustReadPkgFile`; kept the three tiny
authorizer impls separate (clearer than a parameterized one)
- [x] No security issues introduced — fail-closed by construction; 403 body stays
coarse (`commandDenyReason`); override is operator-only, never request input
- [x] No silent failures (select/construct errors surfaced + fatal at boot)
- [x] No debug code left behind
