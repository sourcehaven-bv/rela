---
id: RR-YI8W
type: review-response
title: Test name convention drift
finding: 'runtime_test.go:2330: TestReadBindings_PropagateCallerContext. Codebase mostly uses TestThingName_Scenario style (e.g. TestWithContext_CancellationInterruptsBusyLoop). TestReadBindings_UseCallerContext would read more like the neighbors.'
severity: nit
resolution: Renamed to TestReadBindings_UseCallerContext to match the codebase TestThingName_Scenario convention (verb-form scenario).
status: addressed
---
