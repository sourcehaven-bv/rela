---
id: IMPL-PSKK
type: implementation-checklist
title: 'Implementation: Lift workspace.AttachFile / ListAttachments to internal/attachment'
status: done
---

## Implementation

- [x] Unit tests written for new code (4 tests in attachment, 2 in filename)
- [x] Integration tests via CLI's cliAnalyze interface
- [x] Edge cases handled (nil deps, missing entity, non-file property, orphan file on UpdateEntity failure)
- [x] Code follows project patterns (Deps bundle, nil-rejecting constructor)
- [x] No silent failures

**Summary of changes:**

- `internal/attachment/attachment.go` (NEW, ~140 LOC) — `Service` with `Deps`/`New`/`Attach`/`List`. Types renamed `Info`/`Result` per revive (no stutter).
- `internal/attachment/filename.go` (NEW, ~40 LOC) — `findFileProperty` (now alphabetically deterministic), `contentTypeForName`.
- `internal/attachment/attachment_test.go` + `filename_test.go` — tests migrated from workspace; new `TestService_New_RejectsNilDeps` exercises the constructor.
- `internal/cli/cli_wiring.go` — `cliAnalyze.AttachFile`/`ListAttachments` signatures take `ctx context.Context` and return `*attachment.Result`/`[]attachment.Info`. `cliServices` grows `attachment *attachment.Service`. New `newCLIServicesFromWorkspace` helper shared by production wiring and test fixture.
- `internal/cli/test_helpers_test.go`, `export_test.go`, `rename_test.go` — fixtures use `newCLIServicesFromWorkspace` helper.
- `internal/cli/attach.go` / `attachments.go` — call sites pass `cmd.Context()`.
- `internal/workspace/attachment.go` + `_test.go` + `attachment_filename_test.go` DELETED.
- `.go-arch-lint.yml` — `attachment` component added; cli gains `attachment` dep; `attachment.mayDependOn: entitymanager / metamodel / store`.

**Cranky review (round 1) dispositions:**

| # | Severity | Disposition | Resolution |
|---|----------|-------------|------------|
| 1 | significant | addressed | `Attach`/`List` take `ctx context.Context` instead of `context.Background()` — cancellation propagates |
| 2 | significant | addressed | UpdateEntity-failure error message names the orphaned file path + cleanup instruction |
| 3 | minor | addressed | cliAnalyze doc tweaked to not claim past-tense merge |
| 4 | minor | addressed | `storeStub` doc explains its precise role |
| 5 | nit | wont-fix | panic vs t.Fatal heterogeneity — out of scope |
| 6 | nit | wont-fix | redundant ext == "" early return reads clearer |
| 7 | minor | addressed | `TestContentTypeForName` map → slice for `-run` selection |
| 8 | significant | addressed | `entity not found: %s` → `get entity %s: %w` (proper error wrapping); test asserts `errors.Is(err, store.ErrNotFound)` |
| 9 | leverage | deferred | narrow `entityUpdater` interface for Deps — revisit after TKT-04YA/TKT-B01S |
| 10 | minor | addressed | package doc explains overwrite + single-attachment-per-property semantics |
| 11 | significant | addressed | `findFileProperty` sorts alphabetically (was non-deterministic via map iteration) |

**Manual verification:**

- `go build ./...` — clean
- `go test -race ./...` — all packages pass
- `just lint` — 0 issues
- `just arch-lint` — OK
- `just ci` — full pipeline (frontend, e2e, docs)

**Acceptance criteria:**

1. ✅ `internal/attachment` package exists with `Service`, `New`, `Attach`, `List`, `Info`, `Result`
2. ✅ `internal/workspace/attachment*.go` deleted
3. ✅ `cli_wiring.go` uses `attachment.Result`/`Info` types
4. ✅ Subcommands compile via interface contract
5. ✅ All checks green
