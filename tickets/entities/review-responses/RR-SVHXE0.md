---
id: RR-SVHXE0
type: review-response
title: Config allowlist must cover leaf keys inside each construct
finding: 'The plan allowlisted whole subtrees (entities, validations, automations, lists) and blocked only permission/script/command. Real execution keys sit inside them: property scan_cmd and transform[].cmd (metamodel/types.go:808,826), validation lua/lua_file/lua_args (types.go:172-181), automation action lua/lua_file/allow_acl_bypass/capabilities, list export_render (dataentryconfig/config.go:907). config:edit could escalate to command or Lua execution.'
severity: critical
resolution: 'Plan changed: the allowlist is per construct and per key; any key not listed is locked (create or change refused with 422 naming the path; removing a whole item is allowed). A guard test walks the yaml tags of every struct reachable from metamodel.Metamodel and dataentryconfig.Config and fails on any key the allowlist does not classify. AC10 now covers scan_cmd, transform.cmd, validation lua/lua_file, allow_acl_bypass, capabilities and export_render.'
status: addressed
---
