---
id: IMPL-7CSLLNT
type: implementation-checklist
title: 'Implementation: Headless rela-server build for sqlite and postgres in releases'
started: "2026-11-11"
completed: "2026-11-11"
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

- [x] ~~Using fixture builders or factories for test data~~ (N/A: routing tests over the existing newTestAppV1 fixture)
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**
- AC1: `go build -tags sqlite,headless` and `-tags postgres,headless` of
`./cmd/rela-server` succeed. Sizes: sqlite 82.4 MB → 74.6 MB, postgres 69.9 MB →
62.0 MB. `strings` finds no `static/v2/assets/` in the headless binary;
`scripts/check-embedded-spa.sh binary` fails on it, as the release's inverse
guard expects.
- AC2: live `rela-server` headless sqlite on a temp project: `/` 200 with
`Link: </api/v1/_openapi.json>; rel="service-desc"` and the text body;
`/api/v1/_schema` 200; `/assets/x.js`, `/static/favicon.svg`, `/requirements`
404. Pinned by `TestHeadless_Router`, `TestHeadlessRoot`,
`TestHeadless_NoEmbeddedUI`, `TestHeadless_EditorAssetsNotFound`.
- AC3: untagged `go test ./internal/dataentry/ ./cmd/rela-server/
./internal/tenanthost/` pass.
- AC4: `goreleaser check` validates the config; snapshot release run for the
archives.
- AC5: CI `headless` job and headless lint step added; `go vet -tags headless`
over all packages except rela-desktop passes locally. The compile-time guards
in rela-desktop and rela-docs were dropped after CI: the tagged-file check
(`scripts/check-tagged-tests.sh`) vets every package under each tag, and both
binaries already refuse to serve without the web UI through
`dataentry.CheckEmbeddedSPA` at runtime.
- Edge cases: HEAD `/` 200 without body; POST `/` 404; `/index.html` 404.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
