---
id: DOCS-ZBB8MTY
type: docs-checklist
title: 'Docs: TKT-3RWQ1PK'
started: "2026-11-14"
completed: "2026-11-14"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious: why `-lua-http-allow` is refused outside tenant mode, why mail reads the unwrapped host config, why `string.format` parses the format itself, the embedded IPv4 forms the address check reads
- [x] Function/type docs if public API: godoc on `appbuild.TenantProfile`, `appbuild.WithWithheldHostConfig`, `lua.SetHTTPPolicy`, `lua.ErrHostConfigWithheld`, `script.RequireHostConfig`, `dataentry.DisableGitSync`, `dataentry.SetLuaHost`; `just comment-lint` clean

## Project Documentation

- [x] ~~README updated (if applicable)~~ (N/A: no project-level change)
- [x] ~~CLAUDE.md updated (if new patterns)~~ (N/A: no new cross-cutting convention)
- [x] Help text accurate (if CLI changes): `-lua-http-allow` and `-tenant-shared-host-config` flag help in `cmd/rela-server/tenantmode.go`; the CLI reads `RELA_LUA_HTTP_ALLOW` and `RELA_TENANT_SHARED_HOST_CONFIG`, documented in the server security guide

## User-facing Documentation

- [x] `GUIDE-server-security` (tenant mode as the SaaS profile, withheld host config, its behaviour with `or` and `pairs`, known residue for mail and flows), `GUIDE-lua-scripting` (HTTP destination policy, memory bounds, `string.format` rules, withheld secrets) and `GUIDE-postgres-backend`; regenerated with `just docs`

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: the release changelog is generated from commits)
- [x] ~~API docs updated (if applicable)~~ (N/A: no API shape change; the new 403 code `commands_disabled_in_tenant_mode` is documented in the server security guide)
