---
id: DOCS-STKPI0E
type: docs-checklist
title: 'Docs: TKT-TNPRV8'
started: "2026-11-14"
completed: "2026-11-14"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious: the trust assumption at `tenanthost.provisioning`, why writes during provisioning get 503 rather than 202, the failure and operator holds, why a run uses its own context
- [x] Function/type docs if public API: package doc of `internal/tenanthost` (fail-closed answers, provisioning, suspension sync) and `internal/idporg`; godoc on `ErrNeedsOperator`, `ProvisionConfig`, `StatusSync`; `just comment-lint` clean

## Project Documentation

- [x] ~~README updated (if applicable)~~ (N/A: no project-level change)
- [x] ~~CLAUDE.md updated (if new patterns)~~ (N/A: no new cross-cutting convention)
- [x] Help text accurate (if CLI changes): `-tenant-provision`, `-tenant-provision-rate` and `-tenant-idp-org-api` flag help in `cmd/rela-server/tenantmode.go`; the flags are refused outside tenant mode

## User-facing Documentation

- [x] `GUIDE-postgres-backend` (tenant provisioning modes, eager and lazy paths, status route, rate cap, needs-operator table, IdP suspension sync, erased-tenant tombstone) and `docs/idp-webhook-provisioning.md`; regenerated with `just docs`
- [x] Known limitation documented in the guide: the SPA does not handle the provisioning state yet

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: the release changelog is generated from commits)
- [x] ~~API docs updated (if applicable)~~ (N/A: `GET /api/v1/_tenant` and the provisioning answers are a host route outside the tenant OpenAPI; documented in the postgres backend guide)
