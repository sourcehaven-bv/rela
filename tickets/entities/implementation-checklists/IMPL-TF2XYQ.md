---
id: IMPL-TF2XYQ
type: implementation-checklist
title: 'Implementation: Rename metamodel.yaml to schema.yaml with backward-compatible dual-name discovery'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] Integration tests written (full flow, not just units)
- [x] Feature implemented
- [x] All edge cases from planning handled

**New tests:**

| Test | Covers |
|---|---|
| `internal/project/schema_name_test.go` — `TestDiscoverSchemaFileName` | AC-1/2/3: table-driven over new-only, legacy-only, both-present |
| `TestDiscoverNearestRootWins` | per-level name check; a two-pass walk would open the wrong project |
| `TestDiscoverRelaMarkerWithoutSchemaFile` | AC-4 empty state (RR-JANQG7) |
| `TestSchemaFileAtIgnoresDirectories` | negative: a *directory* named schema.yaml is not a marker |
| `TestExistsAcceptsLegacyName` | the guard behind the init refusal |
| `internal/project/warn_test.go` — `TestWarnIfLegacySchema` | AC-9: warns once, names `rela migrate`, silent when current, nil-safe |
| `internal/projectsetup/schema_rename_test.go` — `TestInitializeRefusesExistingSchema` | **AC-7 / RR-NRFBZE critical**: refuses on either name AND asserts no stray schema.yaml written |
| `TestInitializeCreatesNewName` | AC-7 happy path |
| `TestMigrateRenamesLegacySchema` | **AC-6 / RR-ZB1FJB**: renames, keeps the file in the migration set, second run is a no-op |
| `TestMigrateRefusesWhenBothPresent` | schema.yaml survives byte-for-byte |
| `internal/renametype/legacy_schema_test.go` — `TestRenameTypeWritesBackToLegacyName` | **AC-5 load-bearing**: edits metamodel.yaml in place via real `Discover`, creates no stray schema.yaml |

**Integration:** `TestLoad_AllShippedMetamodels` glob widened to `*schema*.yaml`
— it had silently gone to zero coverage after the in-repo renames (its
`len(paths) == 0` guard caught it). Now loads all six shipped projects.

## Manual Verification (REQUIRED)

- [x] Feature tested end-to-end manually
- [x] EACH acceptance criterion verified
- [x] Verification evidence documented

Built a real binary and drove actual projects in `/tmp`:

| AC | Evidence |
|---|---|
| 1 | fresh project loads, no warning |
| 2 | legacy project: `warning: metamodel.yaml is deprecated … run `rela migrate` to rename it to schema.yaml`, printed once, then loaded 24 entity types |
| 3 | dir with real `schema.yaml` + a 0-type decoy `metamodel.yaml` → loaded **24 types**, proving the decoy was ignored |
| 4 | no-project dir → `no project found: run 'rela init' to create one` |
| 5 | covered by the in-place rewrite test (real `Discover`, no stray file) |
| 6 | `migrate --check` → `metamodel.yaml needs renaming to schema.yaml`, **exit 1**; `migrate` → `Renamed metamodel.yaml → schema.yaml`; dir then holds only `schema.yaml`; re-read warns no more |
| 7 | `rela init` → `Created schema.yaml`. In a legacy dir → refused: `project already initialized (metamodel.yaml exists) — run `rela migrate` to rename it to schema.yaml`, and **no schema.yaml was created** |
| 8 | MCP `tools/list` lists BOTH `get_schema` and `get_metamodel`; `tools/call get_metamodel` returned real entity data (`isError: None`) |
| 9 | warning printed once per process, from the shared appbuild startup path |

**Bug found during manual verification (not caught by tests):** `rela migrate`
renamed the file but printed `No migrations needed.` — a silent layout change.
Fixed by reporting `Renamed X → Y`, and by teaching `--check` to flag a pending
rename and exit non-zero (added `projectsetup.LegacySchemaPending`) so CI
catches it. This is exactly the "no silent changes" gap manual testing exists to
find.

## Quality

- [x] Code follows project patterns
- [x] No silent failures (errors surfaced, not just logged)

- `just lint` → 0 issues (fixed misspell US-spelling, gofmt, govet shadow, gocritic, whitespace in new code)
- `just arch-lint` → OK. Note: an initial attempt to use `project.SchemaFile`
inside `internal/docscapture` was **rejected by arch-lint**; reverted to
literals with a comment explaining why, rather than weakening the boundary.
- `just plimsoll` → clean
- `just coverage-check` → PASS (total 77.1%)
- `just lint-md` → 0 issues
- `go test ./...` → all green
- `just docs` regenerated; diff is rename-only plus the deprecation note

## Scope delivered beyond the original plan

- `rela migrate --check` rename detection (found in manual verification)
- `Context.Exists` widened to both names (same data-loss class as init)
- `internal/dataentry` `projectInfo()` now reports the *resolved* filename —
it is handed to external commands, which would fail opening a name that isn't
there
- Desktop picker de-duplicates a dir holding both names
- Five in-repo projects renamed via `git mv` (history preserved)
