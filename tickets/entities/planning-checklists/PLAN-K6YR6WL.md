---
id: PLAN-K6YR6WL
type: planning-checklist
title: 'Planning: Peer identity and credentials'
started: "2026-11-08"
completed: "2026-11-08"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

The ticket first prescribed `system:<peer>` with the token store. Issue #2119
showed that cannot work: `system:` is refused on every request path, and the
token store holds outgoing OAuth tokens. DEC-81161C3 records the corrected
choice: a peer key verified by rela at the request edge, stamping a reserved but
ordinary principal `peer:<name>`.

**Scope:**

- In: `peer:` reserved prefix and `peer` principal type; key format, digest and
a reloading keyring (`internal/peerkey`); `peer_key.<name>` digests in
`.rela/secrets.yaml`; the peer-key edge in data-entry (401 on a bad key,
read-only methods, `Vary: Authorization`); wiring in rela-server; `rela peer key
create`; `rela reconcile diff --token-secret`; docs (server-security,
acl-security reconcile section, cli reference); RR-KT1PVVG; issue #2140 docs
fix.
- Out: `peers.yaml` and the apply side (TKT-I8L1EQU); key expiry dates;
per-key scopes beyond the ACL; multi-tenant (`tenant`) wiring; a UI.

**Acceptance Criteria:**

1. A peer key reads only what its role allows: dataentry test with an
acl.yaml granting `peer:acme` read on one type; list of the other type is
empty/404 and `_reconcile` counts only the allowed rows.
2. A wrong, revoked or malformed key answers 401 in header mode and JWT mode;
never falls through (test with a valid header user plus a bad peer key).
3. A peer key cannot write: POST/PATCH/DELETE answer 403 `peer_key_read_only`.
4. `X-Forwarded-User: peer:acme`, `$RELA_DATAENTRY_USER=peer:acme` and a JWT
`sub` of `peer:acme` are refused as reserved.
5. No key value in logs, config or audit: log lines carry the peer name only;
test captures slog output on a bad key and asserts the key is absent.

## Research

- [x] For larger features: run `/research` to create a structured research doc
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** RES-G8CYB1R (feature research); the identity choice is in
DEC-81161C3.

**Existing Solutions:**

- JWT gate (`internal/dataentry/jwtgate.go`): fail-closed 401, no explanation
in the body, reason logged. The peer edge copies this shape.
- Webhook principal (`principal.Webhook`, `webhook_routes.go`): a name derived
type (`PrincipalType`) and the reserved-prefix lesson of RR-ZVUHXO. Not an auth
precedent: webhooks are unauthenticated by design.
- `stampAuditPrincipal` + `principal.IsReserved`: the one boundary where request
input becomes a principal.
- `access.Opener`: resolves the stamped principal once per request.
- `secrets` package: `.rela/secrets.yaml` and systemd credentials.
- Prior art elsewhere: GitHub and Stripe style prefixed API keys, stored as a
SHA-256 digest (high-entropy keys need no slow hash), prefix for secret
scanning.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

1. `principal`: `PeerPrefix = "peer:"` added to `IsReserved`; `TypePeer`;
`Peer(name, tool)` constructor via `VerifiedFrom` with the type claim;
`PrincipalType` derives `peer` from the name; `ValidPeerName` (the integration
grammar); `ValidateRunAs` refuses `peer:`.
2. `secrets`: `PeerKeyPrefix = "peer_key."`, `ScriptReadable` refuses it,
`PeerKeyDigests(relaDir)` returns name to digest text from the global section.
3. New leaf `internal/peerkey`: `Prefix = "relapeer_"`; `Generate`, `Digest`;
`Keyring` built from name to digest (validates names, digest format,
duplicates); `Source` re-reads through a loader at most every 10 s, publishes
with `atomic.Pointer` and serializes reloads with a mutex; a load error fails
closed (no key verifies) and is logged by name, never value.
4. `dataentry`: consumer-side `PeerKeys` interface (`IsPeerKey`, `Verify`);
`SetPeerKeys(a, k)` package function (App is at its method cap); `peerKeyEdge`
wraps outside `stampAuditPrincipal` and hands a verified request to the handler
below the JWT gate, so the stamped `peer:<name>` goes through
`stashReplayPrincipal` and `attachACLRequest` like any other. Only `/api/`
paths; a credential without the `relapeer_` prefix takes the normal chain.
Methods: GET, HEAD, and POST on `/api/v1/_reconcile`.
5. rela-server `buildApp`: `SetPeerKeys` with a source over the project's
secrets.
6. CLI: `rela peer key create NAME` prints the key on stdout and the
secrets.yaml line on stderr; `rela reconcile diff --token-secret NAME`.

**Files to modify:** `internal/principal/principal.go`,
`internal/secrets/secrets.go`, `internal/peerkey/*` (new),
`internal/dataentry/peerkey_edge.go` (new), `internal/dataentry/router.go`,
`internal/dataentry/app.go`, `cmd/rela-server/serve.go`, `internal/cli/peer.go`
(new), `internal/cli/reconcile.go`, `internal/cli/root` registration,
`.go-arch-lint.yml`, docs-project guides (server-security, acl-security,
cli-reference), generated `docs/`.

Alternatives rejected: see DEC-81161C3 (system: principal, webhook-style route
identity, JWT-only, proxy header).

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

- `Authorization` header: untrusted. Prefix check, length cap (128 bytes),
base64url alphabet; digest lookup. Any failure is 401 with an empty detail.
- `.rela/secrets.yaml` `peer_key.*`: operator input. Name grammar allowlist,
`sha256:` plus 64 hex; duplicates refused. A bad file fails closed.
- `rela peer key create NAME`: name grammar.

**Security-Sensitive Operations:**

- Key generation: `crypto/rand`, 32 bytes.
- Verification: SHA-256 of the presented key, map lookup on the digest; a
timing difference reveals nothing about a key that hashes elsewhere.
- Namespace: `peer:` reserved at every request entry point.
- Read-only by method, independent of acl.yaml, to limit a leaked key.
- No key value is logged, echoed or written to the audit log.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** AC1 to AC5 as listed above, in `internal/dataentry` tests
with a real router over an in-memory store and an acl.yaml.

**Edge Cases:** an empty or whitespace credential; `Bearer` with a JWT that
starts differently (normal chain); `relapeer_` with a short or long body;
non-base64 characters; keyring reload after removing a key (revocation within
the interval, injected clock); unreadable or malformed secrets file (fail
closed); two names with one digest; a peer key on a non-API path (ignored).

**Negative Tests:** 401 for wrong key, revoked key, malformed key; 403 for a
write method; 403 for asserting `peer:` via header; reserved JWT subject.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

- A third identity source could weaken the header/JWT exclusivity. Mitigation:
a presented peer key decides alone, and a bad key is 401.
- An IdP subject starting with `peer:` stops working. Mitigation: upgrade note
in server-security.
- A leaked peer key reads until revoked. Mitigation: digest-only storage,
10-second reload, read-only methods, ACL role.

Effort: m.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:** GUIDE-server-security (peer keys, reserved `peer:`),
GUIDE-acl-security (reconcile section, #2140 fix), GUIDE-cli-reference (`rela
peer key create`, `--token-secret`).

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** see the review responses linked to the ticket.
