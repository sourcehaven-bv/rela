---
id: RR-Z6UDTW
type: review-response
title: MCP lua_eval can write external refs
finding: WriteExternalRefs is set for every Lua runtime, including client-chosen lua_eval code.
severity: significant
resolution: 'Ref writes need lua.WithExternalRefWrites(), granted only to scheduled, action, automation and rela script runtimes. Tests: TestExternalRefWrites_OnlyWhenGranted, TestLuaEval_ExternalRefWriteRefused.'
status: addressed
---
