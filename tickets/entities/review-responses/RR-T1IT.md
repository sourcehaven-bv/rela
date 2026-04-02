---
id: RR-T1IT
type: review-response
title: Missing tests for Lua automation execution in workspace
finding: The workspace test file has no tests for executeLuaActions(), loadLuaScript(), or executeLuaCode(). These are critical paths that execute user-provided code. While the runtime itself is tested in lua/runtime_test.go, the integration between automation and Lua execution is untested. The path traversal protection via os.OpenRoot is untested in the automation context, the entity/old_entity global injection is untested, and error handling paths are untested.
severity: critical
resolution: 'Added comprehensive Lua automation tests in workspace_test.go: TestLuaAutomation_InlineCode, TestLuaAutomation_EntityGlobals, TestLuaAutomation_OldEntityGlobal, TestLuaAutomation_LuaFilePathTraversal, TestLuaAutomation_LuaFileMissingExtension, TestLuaAutomation_LuaExecutionError. Also fixed a bug where UpdateEntity was applying side effects before writing the entity, causing Lua changes to be overwritten.'
status: addressed
---
