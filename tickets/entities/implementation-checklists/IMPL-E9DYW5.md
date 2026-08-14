---
id: IMPL-E9DYW5
type: implementation-checklist
title: 'Implementation: ICS feed field redaction'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] ~~Integration tests~~ (N/A: the change is one call at the mapping
chokepoint; the three unit tests drive the real `declarativeFeed.List` path)
- [x] Happy path implemented
- [x] Edge cases from planning handled — the filter/redaction ORDER, and the
date-property case where redaction removes the event entirely
- [x] Error handling in place

## Test Quality

- [x] Using fixture builders for test data (`mkTask`, `fakeSource`)
- [x] No hardcoded values in assertions when the object is in scope
- [x] Only specifying values that matter for the test
- [x] Each test verified to FAIL against the fix being removed, including an
ordering sabotage (redact-before-filter) for the membership test

## Quality

- [x] Code follows project patterns — mirrors `affordanceService.copyVisibleProperties`
- [x] Checked for DRY opportunities
- [x] No security issues introduced
- [x] No silent failures
- [x] No debug code left behind

## Automated checks

- `go test ./internal/dataentry/` — PASS (one pre-existing unrelated failure,
`TestBuiltCSSIsLayered`, from stale frontend build artifacts on develop)
- `golangci-lint run ./...` — 0 issues
- `just arch-lint` / `just plimsoll` — OK
