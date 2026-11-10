---
id: DOCS-CUN7DMF
type: docs-checklist
title: 'Docs: TKT-HF5KE12'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious: why the control table lives in `rela_control`, why status changes are compare-and-set, why erase sets `erasing` before the lock, why the registry re-checks after the open, why a revoked in-use resident stays in the map
- [x] Function/type docs if public API: godoc on `ClusterResolver`, `Provisioner`, `Choose`, `Watch`, `WatchDeployment`, `RefuseLegacyMap`, `RefuseTenantDatabase`, `ErrRevoking`; comment-lint `doclink` gate clean

## Project Documentation

- [x] ~~README updated (if applicable)~~ (N/A: no project-level change)
- [x] ~~CLAUDE.md updated (if new patterns)~~ (N/A: no new cross-cutting convention)
- [x] Help text accurate (if CLI changes): `rela-server tenant` usage lists create, show, suspend, resume, erase; `--tenant` help names RELA_TENANT_MODE

## User-facing Documentation

- [x] `GUIDE-postgres-backend.md` "Tenant mode": derived schema and DSN, control table and why it lives in `rela_control`, flags and env, create/show/suspend/resume/erase, upgrade from tenants.yaml, per-org DSN decorator note, erasure steps and guarantee with the PROCEDURE-u020 wording; `GUIDE-acl-security.md` tenant mode paragraph; regenerated with `just docs`
- [x] Security guide: `GUIDE-acl-security.md` updated as above

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: the release changelog is generated from commits)
- [x] ~~API docs updated (if applicable)~~ (N/A: no HTTP API change)
