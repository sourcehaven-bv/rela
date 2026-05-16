---
id: IMPL-JT32
type: implementation-checklist
title: 'Implementation: Lift workspace.RenameEntityType to internal/renametype'
status: done
---

## Implementation

- [x] New `internal/renametype` package with `Service` + `Rename(oldType, newType, newPlural) (int, error)`
- [x] Constructor `New(Deps{FS, Meta, Paths})` rejects nil deps (errors.New, matches attachment style)
- [x] Helpers `rewriteEntityTypeInDir` / `rewriteEntityTypeInFile` / `replaceYAMLType` move with implementation
- [x] CLI wiring (`newCLIServicesFromWorkspace`) constructs the service from `ws.FS()` / `ws.Meta()` / `ws.Paths()`
- [x] `internal/workspace/rename_type.go` + tests deleted
- [x] `.go-arch-lint.yml`: new `renametype` component; `cli` gains it; `renametype` deps = [metamodel, project, storage]
- [x] `go test -race ./...` clean
- [x] `just lint` clean
- [x] `just arch-lint` OK
- [x] `just ci` full pipeline green

## Cranky review disposition

| # | Severity | Status | Notes |
|---|----------|--------|-------|
| 1 | significant | **Addressed** | Wiring conditional → panic at call time. Soft errors invited drift; loud panic surfaces test-fixture gaps unmistakably. |
| 2 | significant | **Addressed** | Template-rename + MkdirAll errors surfaced (not swallowed). Doc comment now explicitly states "NOT ATOMIC" and each error names what already succeeded so operators can re-run. |
| 3 | significant | Won't fix | Deps-access style (local aliases vs inline). Picked local aliases here because `Rename` reads 3 deps × 5 sites; attachment reads each dep 1-2 times. Different complexity warrants different style. |
| 4 | minor | **Addressed** | `0755` → `0o755`, `0644` → `0o644` in implementation. |
| 5 | minor | **Addressed** | Godoc fixed: only step 4 is best-effort; step 2 errors. |
| 6 | minor | **Addressed** | CRLF test case added — documents that the rewritten line loses `\r` while surrounding lines keep theirs. Mixed line endings on output is the *current* behavior; test pins it. |
| 7 | nit | **Addressed** | Comment on zero-value Metamodel/Context in `TestService_New_RejectsNilDeps`. |
| 8 | leverage | Won't fix | YAML AST round-trip would lose byte-for-byte preservation. Trade-off intentional. Not documenting in code (would invite the next dev to chase). |
| 9 | leverage | Won't fix | Service vs free function — keep Service for consistency with attachment. |
| 10 | leverage | Deferred | Fluent test builders. Pattern propagation tracked separately. |
