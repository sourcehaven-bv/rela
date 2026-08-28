---
id: DOCS-EY92SD
type: docs-checklist
title: 'Docs: Postgres derived-schema reconciler (TKT-3Q0GP1)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Godoc on new exported types/functions (DerivedSchemaReconciler, DerivedObjectSpec/Outcome, UniquePropertyError, ErrReconcileBusy, ReconcileDSN)
- [x] Load-bearing doc comments (Reconcile boot-only note; empty-exempt predicate lockstep note; unique.go enforcement note updated)
- [x] ~~Inline comments for non-obvious logic~~ (present: hash determinism, per-rule prefix, try-lock rationale, DDL-injection guards)

## Project Documentation

- [x] `docs/postgres-backend.md` — new "Derived schema (unique constraints)" section: reconcile model, degrade-not-crash, `rela db reconcile [--dry-run] [--show-values]`, operator pre-flight workflow, trust boundary
- [x] `docs/acl-security.md` — provision cross-process caveat now CLOSED on postgres (automatic index); fs/mem caveat remains
- [x] `docs/metamodel.md` — `unique:true` is DB-enforced on postgres, and is string-valued-only
- [x] ~~`docs/cli-reference.md`~~ (N/A: no dedicated db-command section; the postgres-backend guide is the home for `db reconcile`)
- [x] Source edited in `docs-project/entities/guides/`, regenerated via `just docs`

## External Documentation

- [x] ~~README~~ (N/A: internal backend feature, no project-level surface change)
- [x] ~~API docs / OpenAPI~~ (N/A: no new HTTP API; the write 422 is unchanged in shape)
