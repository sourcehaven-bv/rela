---
id: DOCS-Z5S2DL
type: docs-checklist
title: 'Docs: ACL-bound Lua reads + scheduler run_as (TKT-ZF2DTV)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] `lua.ReadDeps.VisibleReader` / `WritePrepStore` godoc carries the read-out vs write-prep boundary and the data-destruction hazard in full
- [x] `lua.EntityReader` godoc explains why the wiring choice IS the read-ACL
- [x] `visibility.ScriptReader` godoc: load-then-Filter rationale, Binder requirement, honest allocation/load costs
- [x] `visibility.DenyReader`/`DenyTracer` godoc: why unattended paths refuse rather than degrade
- [x] `Services.ScheduledLuaWriteDeps` godoc: the field-redaction KNOWN LIMITATION, plainly stated
- [x] `TaskConfig.RunAs` godoc: identity-not-capability, privileges from acl.yaml

## Project Documentation

- [x] CLAUDE.md — new rule: "Never redact a read that feeds a write", citing the guard test

## External / User-Facing Documentation

- [x] `docs/scheduled-tasks.md` — new "Identity and what a task can read (`run_as`)" section: paired acl.yaml example, the empty-reads-on-unassigned-identity failure mode, the field-redaction limitation
- [x] `docs/lua-scripting.md` — new "What a script can read (access control)" section: per-path identity table, behavioral consequences (absent-not-error, peer-gated relations, pruned traversals, unredacted update read), NopACL parity note
- [x] `docs/transforms.md` — export_render residual updated from "open" to **closed**
- [x] ~~docs/metamodel.md, cli-reference.md, README.md~~ (N/A: no metamodel/CLI/project-level change)
