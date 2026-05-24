---
id: IMPL-FL0L
type: implementation-checklist
title: 'Implementation: Build-tag seams in appbuild + cli/mcp_wiring composition roots'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] ~~Integration tests written~~ (N/A: pure refactor — existing
  conformance + integration tests exercise the seam by virtue of
  compiling and running under both build tags)
- [x] Happy path implemented
- [x] Edge cases from planning handled (typed-nil-into-interface
  guarded via `asObserver`/`asMCPObserver`; concurrent close
  protected by `sync.Once`; nil observer dropped at the source)
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

```
$ go build -o /tmp/rs-fs ./cmd/rela-server
$ go build -tags memorybackend -o /tmp/rs-mem ./cmd/rela-server
$ ls -lh /tmp/rs-{fs,mem}
40M /tmp/rs-fs
24M /tmp/rs-mem
$ go list -deps ./cmd/rela-server | grep -c blevesearch
66
$ go list -tags memorybackend -deps ./cmd/rela-server | grep -c blevesearch
0
```

Both builds compile, both `appbuild` + `appbuildtest` test packages
pass under both build tags, `just lint` and `just arch-lint` clean.

## Quality

- [x] Code follows project patterns (check similar code) —
  `appbuildtest` mirrors `internal/store/storetest`; build-tagged
  files use the standard `//go:build !postgres && !memorybackend`
  guard
- [x] Checked for DRY opportunities — `backfill` is triplicated
  across three files (FS appbuild, FS mcp_wiring, test fixture);
  per review, the natural home is `internal/search/backfill.go` but
  the unification is deferred until the two composition roots
  collapse into one (cli/mcp_wiring → appbuild). Three near-identical
  copies behind small naming differences is acceptable for the
  short-term boundary; tracked as follow-up in the ticket.
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned). The one new
  silent path is `mcpWatcher.Start` no-op'ing when the store lacks
  `storeStartStopper` — but it now emits a `slog.Warn` so operators
  see it.
- [x] No debug code left behind
