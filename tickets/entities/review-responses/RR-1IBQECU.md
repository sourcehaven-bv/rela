---
id: RR-1IBQECU
type: review-response
title: Lua HTTP policy not wired everywhere and not fail-closed
finding: The strict Lua HTTP policy was only applied by rela-server. CLI paths that open a tenant (scheduler, stdio MCP with lua_eval, data commands) ran with the default policy, and nothing checked that the strict policy was in effect.
severity: minor
resolution: 'Fixed in 6ed495060 and 3a5b2d9ff: appbuild.TenantProfile is applied in the CLI openProject as well as rela-server. ApplyLuaPolicy returns an error unless lua.CurrentHTTPPolicy reports a strict policy afterwards, and it enables script.RequireHostConfig. Pinned by tenantprofile_test.go and cli/tenant_test.go.'
status: addressed
---
