---
id: IMPL-VMX574
type: implementation-checklist
title: 'Implementation: Enable gosec G705 (XSS), add nosniff to feeds, render help via html/template'
status: done
---

## Development

- [x] Unit tests written for new code — `handlers_xss_test.go` (13 subtests) and
`feed_handler_xss_test.go`
- [x] Integration tests written (test full flow, not just units) — both suites
drive the handlers and assert on the rendered response
- [x] Happy path implemented
- [x] Edge cases from planning handled — mermaid-block escaping, iCalendar
CRLF/`;`/`,` injection, and JSON escaping with a round-trip check proving
escaping doesn't corrupt data
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

`handlers_xss_test.go` — 13 subtests covering every plain-string field,
mermaid-block escaping, and a test pinning that Description/Help remain
raw-markup, so switching them to plain strings fails loudly and forces
re-review.

`feed_handler_xss_test.go` — nosniff on both formats; iCalendar CRLF/`;`/`,`
injection; JSON escaping with a round-trip check.

Both suites were mutation-tested rather than assumed non-vacuous: removing
`nosniff` or unescaping a template field makes the relevant tests fail.

The `metamodel.EntityDef.Description` trust argument was established by
enumerating writers, not by inspection: the only metamodel writers are CLI-only
and none is imported by `dataentry`/`mcp`/`lua`/`rela-server`, and Lua's
`rela.write_file` is confined to `outputDir` by `filepath.IsLocal`.

Static checks: `golangci-lint run ./...` with G705 enabled reports 0 issues;
`go build ./...` and `go test ./internal/dataentry/... ./internal/calfeed/...`
pass including `-race`, re-verified after rebasing onto current `develop`.
