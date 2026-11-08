---
id: IMPL-2UJZNAO
type: implementation-checklist
title: 'Implementation: Peer identity and credentials'
started: "2026-11-08"
completed: "2026-11-08"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code: `internal/peerkey` (format, digest, keyring, reload, fail closed), `internal/principal/peer_test.go`, `internal/secrets/peerkey_test.go`, `internal/cli/peer_test.go`
- [x] Integration tests written (test full flow, not just units): `internal/dataentry/peerkey_edge_test.go` drives the real router with an ACL and the reconcile endpoint, in header, env and JWT mode
- [x] Happy path implemented: a valid key reads as `peer:<name>` under its role, list, single GET, schema and reconcile
- [x] Edge cases from planning handled: malformed, unknown, revoked, lower-case scheme, two Authorization values, bad key next to a valid identity header, reserved name asserted by header/env/JWT, write routes, streams, collab
- [x] Error handling in place (errors surfaced, not swallowed): a key list that fails to load is logged at Error and admits no key; CLI errors name the source, never the value

## Test Quality

- [x] Using fixture builders or factories for test data: `newPeerFixture`, `newTestAppV1`, `localServices`
- [x] No hardcoded values in assertions when object is in scope: the stamped principal is compared with `principal.Peer` output, the digest with `peerkey.Digest(key)`
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

Built `rela` and `rela-server` and ran them on a temporary project with a `note`
and a `secret` type, acl.yaml `"peer:acme": notes-reader`, and
`--principal-header X-Remote-User`:

- `rela peer key create acme > acme.key` printed the key on stdout only and
the `peer_key.acme: sha256:...` line on stderr.
- Peer GET `/api/v1/notes` 200 with the note; `/api/v1/secrets` 200 empty;
`/api/v1/secrets/SEC-R8HB` 404; `/api/v1/_schema` 200; `/api/v1/_events` 403;
POST/PATCH 403.
- `relapeer_AAAA` 401, also next to `X-Remote-User: alice`.
`X-Remote-User: peer:acme` 403 reserved.
- `rela reconcile diff --token-secret source_key --types note,secret` showed
only `secret/SEC-R8HB` as "only here": the peer's view excludes it.
`--token-secret peer_key.acme` refused.
- Removed the `peer_key.acme` line: after 11 s the key answered 401 without a
restart. The log line "peer key refused" carried no key; grep for the key in the
server log found nothing.

## Quality

- [x] Code follows project patterns (check similar code): consumer-side `PeerKeys` interface, package-function setter (plimsoll), `atomic.Pointer` publish plus mutex reload, injected clock
- [x] Checked for DRY opportunities: `principal.ReservedPrefixes` replaces the per-message prefix lists; `stampAuditPrincipal` is the one identity stage
- [x] No security issues introduced: `system:` guard unchanged; digest-only storage; constant-time not needed for a SHA-256 map lookup of a 256-bit key; route allowlist
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
