---
id: DOCS-VIVAYBL
type: docs-checklist
title: 'Docs: TKT-M3SHYE0'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious: the tenant gate's rules (package comment in `internal/cli/tenant.go`), why validate is tenant-free, why `db` commands pin the schema and guard a bare DSN, and why callers close the `tenant.Opened` rather than the services
- [x] Function/type docs if public API: godoc on `tenant.ConfigPath`, `LoadDeployment`, `Config.Select`, `SelectIn`, `OpenOne`, `Pin`, `CheckBareTarget`, `Supported` and the new sentinel errors; comment-lint `doclink` gate clean

## Project Documentation

- [x] ~~README updated (if applicable)~~ (N/A: no project-level change)
- [x] ~~CLAUDE.md updated (if new patterns)~~ (N/A: follows the existing identity-at-the-edge rule; no new cross-cutting convention)
- [x] Help text accurate (if CLI changes): `--tenant` help names tenants.yaml and that data commands need it there

## User-facing Documentation

- [x] `GUIDE-postgres-backend.md`: new "Operator commands in tenant mode" section (`--tenant`/`$RELA_TENANT`, fail-closed rules, tenant-free commands, `db` commands, one scheduler process per tenant and why, no org claim, rela-docs refusal); "Not yet in tenant mode" no longer lists the scheduler; regenerated with `just docs`
- [x] ~~Security guide~~ (N/A: no ACL rule change; isolation is the tenant store handle, documented in the postgres guide)

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: the release changelog is generated from commits)
- [x] ~~API docs updated (if applicable)~~ (N/A: no HTTP API change)
