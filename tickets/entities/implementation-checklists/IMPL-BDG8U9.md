---
id: IMPL-BDG8U9
type: implementation-checklist
title: 'Implementation: Remote MCP part 2 — serve the MCP endpoint over Streamable HTTP'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code — `mcp_http_test.go` (mount, opt-in, CSRF, attribution), `mcp_test.go` (wiring, no-op watcher), `principal_test.go` (ctx-identity precedence), `acl_test.go` (two principals, different rows)
- [x] Integration tests written — `TestRemoteMCP_ReachableThroughRouter` drives the real `App.NewRouter()` chain end to end
- [x] Happy path implemented — `POST /api/v1/_mcp`, opt-in via `-mcp`, stateless streamable HTTP
- [x] Edge cases handled — non-POST still authenticates; a sibling path (`/api/v1/_mcp-other`) does NOT get MCP attribution; nil/erroring handler factory refused at startup
- [x] Error handling in place — `SetRemoteMCP` returns errors for nil factory, nil handler, factory error and missing JWT gate; `rela-server` exits rather than booting without the feature the operator asked for

## Test Quality

- [x] Using fixture builders — `newTestAppV1`, `appbuildtest.New`, `gatedServer`
- [x] No hardcoded values in assertions — ids/titles come from the fixture constants
- [x] Only specifying values that matter — the stub records principals; protocol handling is left to the SDK's own tests
- [x] Interpolated values constructed from objects — `MCPPath` is referenced, never re-typed as a literal
- [x] Property comparisons use original object — `principal.Equal`, not field-by-field string compares

## Manual Verification

- [x] Feature manually tested end-to-end — read the go-sdk source to confirm `connectStreamable(req.Context(), ...)` propagates the HTTP request ctx, which the per-caller identity design depends on
- [x] Each acceptance criterion verified — 6 of 8 done and test-pinned; AC 7 and AC 8 explicitly deferred (RR-P34E8J, RR-PQ5UN1) rather than silently skipped
- [x] Edge cases verified — mutation-tested every security claim: removing the JWT-gate refusal, reverting the tool attribution, the naive composite-literal Tool swap (drops roles), unconditional principal overwrite (collapses identities), and an ungated reader (cross-tenant leak). Each makes a specific named test fail

## Quality Checks

- [x] Linter passes — `golangci-lint` 0 issues
- [x] Type checker passes — `go build ./...` and `go vet` clean
- [x] Coverage thresholds met — `just plimsoll` and `just arch-lint` OK; `mcp.Server` load line raised 48 → 49 with justification for the one added method
- [x] No debug artifacts left behind
