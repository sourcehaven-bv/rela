---
id: IMPL-J5RMKW
type: implementation-checklist
title: 'Implementation: Replace the shape-hash migration chain with a committed applied-list; enable data-only migrations'
status: in-progress
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] Integration tests written (test full flow, not just units)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] Error handling in place (errors surfaced, not swallowed)

Five commits on `tkt-xcj0y2-migration-applied-list`:

1. `StateStore` seam + `MigrationName` + conformance harness + file/memory backends
2. Track migrations by name (file format, `Resolve`, gate split, CLI, `baseline`)
3. postgres + sqlite backends, `LegacyBridge`
4. Docs (data-migration, cli-reference, postgres-backend guides; CLAUDE.md)
5. CLI tests

## Test Quality

- [x] Using fixture builders or factories for test data
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

`migstatetest.RunAll` is the shared contract all four backends run against, so a
divergence fails per-backend rather than being discovered in production.
Assertions compare against `metaV1().ShapeProjection().Hash()` and
`f.ToProjection.Hash()` rather than literal digests, so a deliberate projection
change does not require editing constants.

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

Built `rela` and ran a real filesystem project through the whole cycle:

1. **Data-only migration** (AC-1) — wrote `20260919150000-backfill-owner.yaml`
with identical `from_projection`/`to_projection` and no hashes. It parsed,
dry-ran with per-step counts, applied, and was recorded by name. Under the
previous model this file was rejected at parse time.
2. **Re-run skips** — second `migrate data` reported in-sync, no second entry.
3. **Un-baselined refusal** (AC-5) — deleted `applied.json` with the migration
still present; `migrate status` refused with the actionable message rather than
silently re-baselining.
4. **`migrate baseline`** — dry-run listed the file, `--apply` recorded it, and
status returned to in-sync.
5. **Committed location** — `applied.json` landed in `migrations/`, not under
the gitignored `.rela/`.

Backend conformance: `migstatetest.RunAll` passes for file, memory and sqlite.
**PostgreSQL was run against a real database** (`RELA_TEST_DATABASE_URL` against
a local instance), including `TestTenantIsolation` and
`TestCrossProcessVisibility` — the guarantee `docs/postgres-backend.md`
documents and the reason the record is backend-selected rather than always a
file.

One correction worth recording: an initial attempt to verify AC-4
(server-read-only) by running `rela-server` reported a pass, but the server
exits before wiring in this checkout because the SPA is not built — so that run
proved nothing. The behaviour is genuinely covered by
`TestGate_EvaluateDoesNotPersist` plus `internal/appbuild/datamigration.go`
calling only `Evaluate`, which is what was verified instead.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind

Patterns followed: the `StateStore` seam mirrors `comments.Store` (interface +
per-tier packages + recipe-selected override + conformance harness), the file
backend copies `filecomments`' SafeFS-then-RootedFS ordering, and the database
backends declare their own narrow handle rather than importing `internal/store`
(arch-lint enforces it).

Security (RR-J064T6): migration names are allowlist-validated through a
`MigrationName` type whose field is unexported, enforced on BOTH the directory
listing and the applied list so the two populations share one alphabet.
Rejection is total — no sanitize-and-continue. Lowercase-ASCII-only forecloses
the macOS case-folding and Unicode NFC/NFD collisions. Tests cover traversal,
uppercase, unicode, null bytes, newlines and overlong names.

Not silent: a corrupt `applied.json`, an unknown field, a name outside the
allowlist, and state from a newer `FormatVersion` all ERROR rather than reading
as absent — absence means "un-bootstrapped", which would baseline the store and
strand every pending migration. One deliberate exception, documented at the call
site: a corrupt LEGACY marker falls back to bootstrap, because that path is only
reached when the new record is already empty.

A behaviour the tests pinned rather than assumed: a YAML file in `migrations/`
whose name fails the allowlist fails the whole load loudly. Skipping it quietly
would mean an operator-written migration never runs with nothing saying so.
