---
id: IMPL-HX893M
type: implementation-checklist
title: 'Implementation: JWT identity must fail closed, never downgrade to --principal-header'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Added `jwtauth.ErrKeysUnavailable` + `classify()` splitting JWKS-unreachable from assertion-rejected
- [x] Wired `RefreshErrorHandlerFunc` (was available in `keyfunc.Override`, unset) for the JWKS-refresh operational alert
- [x] Extracted `isAPIPath` shared by `attachACLRequest` and the new gate so the RR-P2M7 bare-`/api` case cannot drift
- [x] Added `requireVerifiedJWT` + `JWTGateConfig` + `App.SetJWTGate` (`internal/dataentry/jwtgate.go`)
- [x] Placed the gate between `attachACLRequest` and `stampAuditPrincipal`, with a CRIT-1-voice comment on why both alternatives are wrong
- [x] Added `validateIdentityFlags` (pure, table-testable) + startup wiring; deleted the obsolete `slog.Warn`
- [x] Closed the adjacent footgun: partially-set `--jwt-*` flags used to silently disable identity
- [x] Rate-sampled the keys-unavailable Error log so a rotation-during-outage cannot flood
- [x] Marked `JWTPrincipalResolver` deprecated (now unwired) rather than deleting exported API

## Quality

- [x] Tests pass (`go test ./...`, full suite)
- [x] Race detector clean (`go test -race` on the three affected packages — the gate has shared sampling state)
- [x] Lint clean (`just lint`, 0 issues)
- [x] `just arch-lint` clean — added `jwkset` to the `jwtlib` vendor bundle (bare path + glob; the module is imported at its root)
- [x] `just plimsoll` clean — bumped the grandfathered `App` pin 131→132 for the one new method
- [x] `just coverage-check` passes (76.3% total)
- [x] New user-facing behavior documented in `docs/server-security.md`
