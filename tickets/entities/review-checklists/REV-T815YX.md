---
id: REV-T815YX
type: review-checklist
title: 'Review: fix policy-less scriptReader test (TKT-U8G4Q0)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`go test -race ./internal/dataentry/` — full package)
- [x] Lint clean (`golangci-lint run ./internal/dataentry/`) — 0 issues
- [x] `gofmt` clean
- [x] ~~`just plimsoll`~~ (N/A: test-only change, no type surface touched)

## Manual Review

- [x] Root cause identified as a semantic conflict (#1204 test vs #1208 named-handle refactor), not a regression — both sides' intent preserved
- [x] New assertion pins the TKT-1WV50C contract (`*visibility.UnrestrictedReader`), keeps the DenyReader fail-closed assertion, and the scriptTracer side untouched
- [x] Pass-through equivalence of the wrapper is already pinned by `internal/visibility/unrestricted_test.go` — no behavioral coverage lost
- [x] ~~`/code-review` agent run~~ (N/A: 8-line test-only diff, reviewed inline)

**Review Responses:** none.
