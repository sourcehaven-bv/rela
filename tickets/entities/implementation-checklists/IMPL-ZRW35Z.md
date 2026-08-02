---
id: IMPL-ZRW35Z
type: implementation-checklist
title: 'Implementation: Enable gosec G704 (SSRF) and annotate operator-configured HTTP destinations'
status: done
---

## Development

- [x] ~~Unit tests written for new code~~ (N/A: no behaviour change — two
annotations and a lint-config edit)
- [x] ~~Integration tests written~~ (N/A: no behaviour change)
- [x] Happy path implemented
- [x] Edge cases from planning handled — the server-response-feeds-request-path
sub-angle was checked explicitly, see evidence
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

Both trust boundaries were traced to their source rather than asserted:

- The AI endpoint's provenance was followed from `.rela/ai.yaml` through
`LoadConfig` / `LoadProvider` to its single non-test call site, and the negative
("Lua cannot override it") was confirmed at two layers — the request type has no
URL field, and the Lua binding parses no URL key.
- One sub-angle checked because a *remote* server's manifest response feeds back
into subsequent request paths: `newRequest` builds paths via `url.URL.JoinPath`
over segments derived from record ids, joining onto the base and escaping each
segment exactly once. A hostile server can influence the *path* but never the
scheme or host, so it cannot redirect requests to a new destination.
Cross-origin redirects are not followed.

Static checks: `golangci-lint run ./...` with G704 enabled reports 0 issues;
`go build ./...` and `go test ./internal/ai/... ./internal/cli/...` pass,
re-verified after rebasing onto current `develop`.
