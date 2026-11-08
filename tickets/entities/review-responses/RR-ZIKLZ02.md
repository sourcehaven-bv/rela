---
id: RR-ZIKLZ02
type: review-response
title: 'Design: tenants share one Lua cache through the base script engine'
finding: OpenAndAssemble copies the base and swaps only the DSN, so every tenant uses base.cfg.ScriptEngine and its process-wide lua.Cache. A rela.cache value stored by a tenant A script is returned to tenant B (script/executor.go, list_document.go keys by script path).
severity: critical
resolution: OpenAndAssemble gives each tenant a new script.Engine (own Lua cache). Pinned by TestTenantIsolation_OwnAuditAndScriptEngine.
status: addressed
---
