---
id: RR-GUBJWXN
type: review-response
title: "Parallel subtests share one mock workspace"
finding: "In validation/lua_test.go the merged Severity tables shared one newMockWorkspace across parallel subtests."
severity: minor
resolution: "Each subtest builds its own workspace again; TestLuaValidation_ReturnValues already shared one on develop."
status: addressed
---

In validation/lua_test.go the merged Severity tables shared one newMockWorkspace across parallel subtests.
