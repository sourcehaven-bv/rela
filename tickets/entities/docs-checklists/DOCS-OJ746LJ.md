---
id: DOCS-OJ746LJ
type: docs-checklist
title: 'Documentation: TKT-TNPRV8'
status: done
---

## Code Documentation

- [x] Comments where logic isn't obvious
- [x] Function/type docs if public API (godoc on tenant.AutoProvisioner incl. the trust assumption, tenant.Sweeper, tenanthost provisioning section, flowrun.CopyRun)

## Project Documentation

- [x] ~~README updated (if applicable)~~ (N/A: no project-level change)
- [x] ~~CLAUDE.md updated (if new patterns)~~ (N/A: no new code pattern)
- [x] Help text accurate (if CLI changes): -tenant-provision flag help lists off, webhook and lazy
- [x] GUIDE-postgres-backend updated (provisioning modes, status endpoint, erased-org tombstone, due-work sweep, shutdown); docs/postgres-backend.md regenerated

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: release notes are generated from merged PRs)
- [x] API docs updated (if applicable): GET /api/v1/_tenant/status documented in the guide
