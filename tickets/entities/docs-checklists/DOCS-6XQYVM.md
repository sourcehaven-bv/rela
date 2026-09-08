---
id: DOCS-6XQYVM
type: docs-checklist
title: 'Docs: Data migration system (TKT-0C57FS)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Package doc: `internal/datamigration/doc.go` — model, trust boundary, relation to the two other "migration" systems
- [x] Godoc on every exported type/function; load-bearing invariants documented at the declaration (idempotency contract on `Step`, two-hash rationale on `ShapeProjection`, capture rationale in `run.go`)
- [x] fsstore type-change contract documented at the storetest case and the fix site

## Project Documentation

- [x] New `docs/data-migration.md` — shape hash, gate tiers, rename blind spot, file format, step reference, Lua contract, workflow, GC/grace, two-hash rationale
- [x] `docs/cli-reference.md` — `rela migrate status|gen|data|gc` section with flags and CI usage
- [x] `docs/postgres-backend.md` — per-tenant migration state, version capture on destructive steps, GC sweep controls
- [x] `docs/metamodel.md` — schema-evolution face in "After Modifying the Metamodel"
- [x] `CLAUDE.md` — data-migration rule block (two hashes, third sanctioned raw-write exception, idempotency, pure-transform Lua)
- [x] `.go-arch-lint.yml` — datamigration component, dependency rules, documented gopher-lua vendor allowance

## External Docs

- [x] ~~Website/blog announcement~~ (N/A: no external docs pipeline in this repo; docs/ is the published surface)
