---
id: DOCS-TNERAS1
type: docs-checklist
title: 'Docs: tenant erasure (TKT-TNERAS)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious (lock order in pinSchema and dropSchema, the public-tables guard, erase order, symlink refusal)
- [x] Function/type docs if public API (tenant.Erase, Erasure, ErasureTarget, AuditRoot, Opened, ErrTenantInUse, ErrPublicHasTables, ErrNotMarkedForErasure, DescribeTarget)

## Project Documentation

- [x] ~~README updated~~ (N/A: erasure is documented in the postgres backend guide)
- [x] ~~CLAUDE.md updated~~ (N/A: no new cross-cutting pattern)
- [x] Help text accurate (rela-server tenant usage text)

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: the repo has no changelog; release notes come from PR titles)
- [x] API docs updated (GUIDE-postgres-backend "Erasing a tenant", regenerated into docs/)
