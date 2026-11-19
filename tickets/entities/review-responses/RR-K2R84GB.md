---
id: RR-K2R84GB
type: review-response
title: Hook error on fs is not reported as KeptEntityError
finding: On a store that cannot roll back, a hook error after the row write returned a plain error although the row stayed.
severity: significant
resolution: 'createCore returns the entity with a createHookError; CreateEntity, CreateLinked and CreateBatch report KeptEntityError/BatchKeptError and run the kept follow-ups. Test: TestCreateHook (hook error on memstore).'
status: addressed
---
