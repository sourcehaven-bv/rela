---
id: IMPL-RALDI7
type: implementation-checklist
title: 'Implementation: Sync 5/5: rela CLI sync client — index, topo-ordered diff, push/pull, manual --force'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code (state, diff, order, client, push, pull, force)
- [x] Integration tests written (httptest fakeServer mirroring the real /api/sync/ contract; full push↔pull convergence)
- [x] Happy path implemented (push/pull create/update/delete)
- [x] Edge cases from planning handled (conflict halt, both-dirty, topo order, idempotent resume, locked records)
- [x] Error handling in place (transport/auth abort the run; conflict/validation halt the record; errors surfaced not swallowed)

## Test Quality

- [x] Using fixture builders (harness, createLocalEntity, memApplier)
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] Feature manually tested end-to-end (`rela sync push/pull --help`; `sync push` with no remote → clear "--remote required" error)
- [x] Each acceptance criterion verified with a test scenario (AC#1 converge, AC#2 mirror, AC#3 conflict+force, AC#6 topo order, idempotent resume, force-unknown error, bearer auth)
- [x] Edge cases manually verified

**Verification Evidence:** `go test -race ./internal/cli/sync/` passes (12 tests
incl. 3 review-regression tests). Coverage 71.0%, above the 50 floor. `rela sync
--help` renders push/pull; `rela sync push` against a project with no remote
returns `sync: --remote base URL is required`.

## Quality

- [x] Code follows project patterns (consumer-side LocalApplier interface + type-assert, mirroring the server side; nil-checking constructors)
- [x] Checked for DRY opportunities (shared orderForApply generic for push+pull; shared engine collaborators)
- [x] No security issues introduced (token header-only, never logged/URL'd; client-side id allowlist mirrors server)
- [x] No silent failures (errors surfaced and returned; unconverged run exits non-zero)
- [x] No debug code left behind
