---
id: IMPL-Z4WAXX
type: implementation-checklist
title: 'Implementation: Fix stale identity assertion in policy-less script-read seam test'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Reproduced the failure locally on `develop` at HEAD (`3974fecb`)
- [x] Traced the cause to #1208 (TKT-1WV50C) changing `a.store` → `visibility.Unrestricted(a.store)`
- [x] Replaced the pointer-identity assertion with a contract assertion (`*visibility.UnrestrictedReader` + pass-through read)
- [x] Confirmed the `scriptTracer` half still returns `a.tracer`, so its identity assertion is correct and left untouched

## Quality

- [x] Target tests pass (`go test ./internal/dataentry/ -run 'TestScriptRead|TestScriptTracer'`)
- [x] Full suite green (`go test ./internal/...`, no failures)
- [x] Lint clean (`golangci-lint run internal/dataentry/...` — 0 issues)
- [x] `go build ./...` clean
- [x] ~~New user-facing behavior~~ (N/A: test-only change, production wiring untouched)
