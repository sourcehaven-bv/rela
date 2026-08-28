---
id: IMPL-4LNI05
type: implementation-checklist
title: 'Implementation: Enable gosec G706 (log injection), scope-exclude dataentry with a tested invariant'
status: done
---

## Development

- [x] Unit tests written for new code — `internal/dataentry/loginjection_test.go`
- [x] Integration tests written (test full flow, not just units) — the AST check
walks the whole package rather than a sampled subset
- [x] Happy path implemented
- [x] Edge cases from planning handled — the check accepts constant *expressions*
(compile-time concatenation of literals), not just bare literals
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

The AST check was mutation-tested: injecting a `slog.Warn(fmt.Sprintf(...))`
fails it, so it is not vacuous.

The handler's escaping guarantee was verified empirically rather than assumed —
an injected `"bob\nlevel=ERROR msg=\"forged\""` comes out escaped on a single
line.

The exclusion was confirmed narrow by planting a temporary canary in
`internal/mcp`: G706 fired there, proving the rule is enforced repo-wide and
suppressed only for `internal/dataentry`. Canary removed. Without this check, a
path-scoped exclusion that accidentally disabled the rule everywhere would look
identical to a working one.

The new test immediately caught two non-constant slog messages that gosec missed
(`app.go`, `jwtgate.go`); both proved benign (compile-time concatenation of
literals, no user data), so the check accepts constant expressions.

No logging code changed; no security control weakened.

Static checks: `golangci-lint run ./...` with G706 enabled reports 0 issues;
`go build ./...` and `go test ./internal/dataentry/...` pass, re-verified after
rebasing onto current `develop`.
