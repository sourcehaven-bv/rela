---
id: RR-8VBAJM
type: review-response
title: Generator in internal/dataentryconfig/gen writes to frontend/ and docs-project/ — an unstated path assumption and an arch-lint/package question the plan leaves open
finding: |-
    Approach §2 places the generator at `internal/dataentryconfig/gen/main.go`, invoked by `//go:generate` and a `just generate-icons` recipe, writing three files including `frontend/src/utils/iconRegistry.generated.ts` and `docs-project/entities/guides/GUIDE-data-entry.md`. Four unresolved problems:

    **1. Repo-root resolution is unspecified.** The generator writes to paths OUTSIDE its own package — two directories up and across into `frontend/` and `docs-project/`. `go:generate` runs with CWD set to the package dir, but `just` recipes run from the repo root, so relative paths differ between the two invocation methods the plan proposes. The existing precedent (`icons_test.go:35`) hardcodes `filepath.Join("..", "..", "frontend", ...)`, which works only because `go test` sets CWD to the package dir. Pick ONE invocation path and state it, or the generator silently writes files to the wrong place depending on how it was run.

    **2. `main` package inside `internal/dataentryconfig/` may trip arch-lint.** `.go-arch-lint.yml` maps the component `dataentryconfig: { in: internal/dataentryconfig }` — note: not `/**`, so a `gen` subpackage is NOT covered by that component and becomes an unmatched package. Compare `apiwire: { in: internal/apiwire/** }`, which explicitly uses the recursive form. `just arch-lint` is in `just check` and is a CI gate. The plan must either add a component entry or place the generator elsewhere (e.g. `cmd/` or `scripts/`), and this needs deciding before implementation, not discovered during it.

    **3. The generator must import the canonical table, which is in the package it generates INTO.** `gen/main.go` importing `internal/dataentryconfig` to read `[]IconDef`, while also writing `icons_gen.go` into that same package, means the generator cannot run when the package it depends on does not compile. Any hand-edit that breaks `icons_gen.go` makes the tool that regenerates it unbuildable — a bootstrap deadlock recoverable only by hand-reverting. This is a known Go generator hazard and needs an explicit answer (e.g. put `IconDef` + the table in a leaf package with no generated file in it).

    **4. Coverage floors.** CLAUDE.md documents `go-test-coverage` package floor thresholds in `.testcoverage.yml`. A new package containing a `main` function is exactly the "new untested package added" case the floors exist to catch. The plan does not mention adding an entry or a `// coverage-ignore:` reason.
resolution: |-
  Addressed in plan. The canonical table moves to a leaf package `internal/dataentryconfig/icondefs` containing no generated file, dissolving the bootstrap deadlock. The generator moves to `cmd/gen-icons` with an explicit `-root` flag and a single invocation path (`just generate-icons`, no `go:generate`). Arch-lint component entries and the coverage-floor decision are now listed in Files to modify.
severity: significant
status: addressed
---

## Recommended fix

**Split the data from the generator.** Put the canonical table in a leaf package
that is never itself generated into:

```
internal/dataentryconfig/icondefs/icondefs.go   ← []IconDef, hand-edited, no generated file here
internal/dataentryconfig/icons_gen.go            ← generated ValidIconNames (imports icondefs)
cmd/gen-icons/main.go                            ← generator (imports icondefs)
```

`icondefs` has no generated content, so it always compiles, so the generator can
always run. This dissolves problem 3 entirely.

Placing the generator under `cmd/` matches where this repo already puts
executables and sidesteps problem 2 — but confirm the arch-lint component map
covers it, since `cmdCli`/`cmdServer`/`cmdDocs`/`cmdDesktop` are enumerated
individually rather than by glob.

**Resolve paths from the repo root explicitly**, not relatively: accept an
`-root` flag defaulting to the value `just` passes (`{{justfile_directory()}}`),
and have `go:generate` pass the same. Then both invocation paths agree.

**Decide the coverage story up front**: either the generator gets a
`.testcoverage.yml` entry, or `main` carries `// coverage-ignore: generator
entry point` per the CLAUDE.md convention for main functions.
