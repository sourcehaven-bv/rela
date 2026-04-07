---
id: IMPL-M7JN
type: implementation-checklist
title: 'Implementation: Harden rela-server against browser-based local attacks'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] Integration tests written (test full flow, not just units)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] Error handling in place (errors surfaced, not swallowed)

## Test Quality

- [x] Using fixture builders or factories for test data
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

Acceptance criteria mapped to tests (all passing):

| AC | Coverage |
|---|---|
| 1: loopback bind default | `cmd/rela-server/main.go` `--bind` flag defaults to `127.0.0.1` (manual: `rela-server` then check `lsof -i :8080` shows loopback only). `TestNewSecurity_NonLoopbackBindOnlyAcceptsItself` covers the allowlist construction. |
| 2: cross-origin POST → 403 | `TestSecuredRouter_RejectsCrossOriginAPI`, `TestRequireSameOrigin_RejectsCrossOriginOnSensitivePath` |
| 3: GET on sensitive endpoint cross-origin → 403 | `TestSecuredRouter_RejectsCrossOriginCommandGET`, `TestRequireSameOrigin_RejectsCrossOriginOnSensitivePath` (includes `/api/command/run` GET case) |
| 4: handleCommandExec POST-only | `TestHandleCommandExec/method_not_allowed` (existing test still asserts 405 on DELETE) plus updated tests POST to the endpoint |
| 5: SSE no CORS reflection | `internal/dataentry/watcher.go` no longer writes `Access-Control-Allow-*`; covered by `TestRequireSameOrigin_RejectsCrossOriginOnSensitivePath` "GET SSE" case |
| 6: spoofed Host → 403 | `TestSecuredRouter_RejectsHostSpoof`, `TestRequireLocalHost_RejectsSpoofedHost` |
| 7: open-file path containment | `TestContainedProjectPath_RejectsTraversal`, `TestContainedProjectPath_RejectsSymlinkOut`, `TestContainedProjectPath_AllowsInsideRoot` |
| 8: open-url scheme allowlist | `TestValidateOpenURL` (file:, javascript:, data:, ftp: rejected; http/https/mailto allowed) |
| 9: relType allowlist | `TestRelationFilePath_PanicsOnTraversal` (defensive panic in path builder) + existing `meta.ValidateRelation` rejects unknown types upstream in `workspace.CreateRelation` |
| 10: WriteCacheFile validation | `TestValidateCacheFilename_Rejects` covers `..`, `/etc/...`, backslash, NUL, double-slash. Existing nested cache callers (`workspace/document.go`) verified still working via existing `TestDocumentDiskCache` |
| 11: server timeouts + streaming preserved | `cmd/rela-server/main.go` sets `ReadHeaderTimeout`/`ReadTimeout`/`IdleTimeout`; `WriteTimeout=0` documented as required for SSE. Existing `TestHandleCommandExec/success_stream` still passes through the SSE `Flusher` path |
| 12: Vite dev server allowlist | `TestRequireSameOrigin_AcceptsExtraAllowedOrigin` (allows `http://localhost:5173` when passed as `--allowed-origin`) |
| 13: rejection response format | `TestSecuredRouter_BlockedResponseFormat` asserts JSON content type, `error: forbidden`, `reason: <rule>`. Log line format verified by inspection in test output |

Edge cases covered explicitly:
- `TestNormaliseOrigin` covers case normalisation, default-port handling (`:80`/`:443`), trailing slash, paths, queries, `Origin: null` (rejected), `file://`/`javascript:`/empty (rejected).
- `TestRequireSameOrigin_FallsBackToReferer` covers Referer fallback when Origin is missing.
- `TestRequireSameOrigin_RejectsMissingOriginAndReferer` covers the both-missing case.
- `TestRequireLocalHost_RejectsSpoofedHost` covers empty Host, missing port, wrong port, evil hostnames.
- `TestSecuredRouter_StaticFilesBypassOriginCheck` confirms the SPA exemption works.

Manual verification:
- `go build ./...` clean
- `just lint` clean (no warnings)
- `go test ./...` all packages pass with `-race`
- `just coverage-check` PASS (total 69.2%, ratchet maintained)
- Visual inspection of `cmd/rela-server/main.go` confirms `Addr: net.JoinHostPort("127.0.0.1", port)` and warning log when bind is non-loopback

## Quality

- [x] Code follows project patterns (check similar code) — middleware composition mirrors existing `reloadLockMiddleware`; security helpers self-contained
- [x] No security issues introduced — every new path validated; tests assert rejections
- [x] No silent failures (errors logged AND returned) — `security.reject` logs at warn level and returns 403 with structured body
- [x] No debug code left behind
