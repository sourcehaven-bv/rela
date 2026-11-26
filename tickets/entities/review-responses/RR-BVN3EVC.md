---
id: RR-BVN3EVC
type: review-response
title: 'Test gaps: include files and metamodel passthrough'
finding: No test for deprecated syntax in schema includes; no test that loadConfig passes a schema.
severity: minor
reason: 'Includes were never migration-checked and that is unchanged (out of scope in the plan). The schema passthrough no longer exists: loadConfig passes nil on purpose.'
status: deferred
---
