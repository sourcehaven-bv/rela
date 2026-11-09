---
id: PLAN-APIH11A
type: planning-checklist
title: 'Planning: Remove peer keys; machine clients authenticate at the identity edge'
started: "2026-11-08"
completed: "2026-11-08"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

The owner decided that rela issues no credentials of its own. Machine clients
authenticate at the identity edge: a verified JWT from Pratique or another OIDC
proxy, or the principal header. A sync client is an ordinary principal, and
`_reconcile` is an ordinary ACL-gated API. PR #2156 added rela-verified peer
keys; this ticket removes them and records the new decision.

**Scope:**

- In: reverse the code of #2156 (`internal/peerkey`, `peerkey_edge.go`, its
wiring in `router.go`, `app.go`, `jwtgate.go` and `cmd/rela-server/serve.go`,
the reserved `peer:` namespace, `TypePeer`, `ValidPeerName`, `ReservedPrefixes`,
the `principal_property` skip, the route allowlist and `peer_route_refused`,
`rela peer key create`, `peer_key.*`, their tests, `.go-arch-lint.yml` and
`.testcoverage.yml` entries, the tenant-mode test that pinned "no peer keys").
Remove the "Peer keys" docs sections.
- In (kept, renamed): `rela reconcile diff --token-secret NAME` reads a global
secret. NAME must start with `remote_credential.`; `secrets.ScriptReadable`
refuses that prefix and `token_key`, so no script can be granted a credential
for a remote. `--token-env` and `--token-file` stay.
- In (kept): the #2140, RR-U6M954C and DOCS-JA3ZYX2 docs fixes (the
`source.pointer` `/ranges/<index>` text).
- In (records): new decision superseding DEC-81161C3; notes on TKT-H91N91L,
TKT-I8L1EQU and RR-KT1PVVG.
- Out: any change to the JWT gate, the header mode, the reconcile handler or
its rate gate; step 8 (TKT-I8L1EQU) code.

**Acceptance Criteria:**

1. No `internal/peerkey`, no `peerkey_edge.go`, no `peer:` reservation, no
`rela peer` command: `go build ./...` and `grep -ri peerkey` in Go code find
nothing.
2. A `peer:acme` user from the principal header is an ordinary user again:
`reserved_principal_test.go` no longer lists `peer:`.
3. `--token-secret remote_credential.src` sends the stored value as the bearer
token; `--token-secret token_key` and `--token-secret api_key` are refused
before any request; a missing secret is an error that names the secret, never a
value.
4. `secrets.ScriptReadable` is false for `token_key` and every
`remote_credential.` name; `capabilities.secrets` naming one fails at schema
load, in a script grant and in `mail.yaml`.
5. Docs: no "Peer keys" section; the reconcile section says the gate is keyed
on the resolved principal and that header-mode callers without identity share
one key, as intended.

## Research

- [x] For larger features: run `/research` to create a structured research doc
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A (a removal; RES-G8CYB1R and DEC-81161C3 hold the options
already compared, including "verified JWT from the IdP", which is now chosen).

**Existing Solutions:**

- The code before #2156 (`02ff7340a^`) is the base for the reverse patch.
- `secrets.ScriptReadable` and its three callers (`lua/capabilities.go`,
`metamodel/capabilities.go`, `mail/config.go`) are the generic protection.
- `cmd/rela-server/serve.go` `appIdentity` got a `peerKeys` flag in #2155;
the flag goes with the feature.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

1. Reverse-apply the non-docs, non-tickets part of `02ff7340a` with
`git apply -3`; resolve the `serve.go` conflict from #2155 by dropping
`appIdentity.peerKeys`; delete `TestTenantMode_IgnoresPeerKeys`.
2. Re-add `secrets.Global` and a `RemoteCredentialPrefix =
"remote_credential."`; `ScriptReadable` refuses `TokenKey` and that prefix. The
three callers get messages that name the generic rule.
3. Re-add `--token-secret` (xor with `--token-env`/`--token-file`) and the
`reconcileServices.Secret` wiring; refuse names outside the prefix.
4. Docs: remove the peer sections from GUIDE-acl-security, GUIDE-server-security,
GUIDE-cli-reference and the GUIDE-postgres-backend mentions from #2155; keep the
`source.pointer` fix; describe the identity-edge way for sync clients; run `just
docs`.
5. Records: decision, notes, RR-KT1PVVG back to deferred-as-intended.

**Files to modify:** `.go-arch-lint.yml`, `.testcoverage.yml`,
`cmd/rela-server/serve.go`, `cmd/rela-server/tenantmode_postgres_test.go`,
`internal/acl/identity.go` and test, `internal/cli/{kong,cli_wiring,reconcile,
secrets}.go` and tests, `internal/dataentry/{app,jwtgate,router}.go` and tests,
`internal/{lua,metamodel}/capabilities.go`, `internal/mail/config.go`,
`internal/principal/principal.go`, `internal/secrets/secrets.go`; delete
`internal/peerkey/`, `internal/cli/peer*.go`,
`internal/dataentry/peerkey_edge*.go`, `internal/principal/peer_test.go`,
`internal/secrets/peerkey_test.go`; four docs-project guides and their generated
`docs/` copies.

Alternatives: `git revert` of the whole commit would also revert the tickets
records and the kept docs fixes; rejected. Letting `--token-secret` read any
global name (as #2156 did) leaves a credential script-grantable if stored under
an ordinary name; the prefix rule closes that.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:** `--token-secret` NAME from the CLI user:
allowlisted by prefix; the value never appears in errors. `secrets.yaml` names
in `capabilities.secrets`: refused at load when not script-readable.

**Security-Sensitive Operations:** the identity edge loses a source. A request
with `Authorization: Bearer relapeer_...` is now handled like any other
credential: the JWT gate verifies it (and refuses it) in JWT mode; header mode
ignores Authorization. `peer:` is no longer reserved, so a proxy or IdP may
assert `peer:acme`; nothing grants that name a special meaning any more.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** AC1: build, `grep`, arch-lint. AC2: reverted
`reserved_principal_test.go`. AC3: `reconcile_test.go` table: secret sent as
bearer to an httptest remote; `token_key`, `api_key`, missing secret, empty
secret, nil Secret func refused; two credential flags refused by kong xor. AC4:
`secrets` `TestScriptReadable`, metamodel/lua/mail tests for a
`remote_credential.` grant. AC5: docs review and `just docs`.

**Edge Cases:** NAME equal to the bare prefix `remote_credential.` (refused,
empty name); value with trailing newline (trimmed); per-script override with the
same name (ignored by `Global`).

**Negative Tests:** as listed in AC3 and AC4.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:** a deployment that already created peer keys loses them; #2156 is
unreleased (merged today), so no release carries it. Missing a reference:
mitigated by grep and arch-lint. Effort: m.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:** GUIDE-acl-security (reconcile section),
GUIDE-server-security, GUIDE-cli-reference (`rela peer key create`,
`--token-secret`), GUIDE-postgres-backend (tenant note); regenerate `docs/`.

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** TKT-TB2B8N5: RR-W983COM (significant, addressed),
RR-0Q2WP03, RR-3KMSQNJ (minor, addressed), RR-37W1P5N (nit, addressed).
