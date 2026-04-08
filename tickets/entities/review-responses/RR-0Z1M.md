---
id: RR-0Z1M
type: review-response
title: 'F16: TestLuaAI_CompleteRejectsNonString asserts only that *some* error occurred'
finding: 'The test would pass if ai.complete({}) raised ''bad argument #1'' OR ''ai.complete is not defined'' OR ''undefined variable ai'' OR any other Lua error. A future refactor that accidentally broke type checking would slip past.'
severity: nit
resolution: Test now also asserts strings.Contains(err.Error(), 'string expected') so a regression that breaks the binding entirely (e.g. ai global no longer registered) cannot accidentally satisfy the assertion.
status: addressed
---
