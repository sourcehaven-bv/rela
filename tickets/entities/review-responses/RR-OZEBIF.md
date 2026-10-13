---
id: RR-OZEBIF
type: review-response
title: 'Missing tests: dry run, store error, own row, faces'
finding: No coverage for these paths.
severity: minor
resolution: Added TestLuaReads_DryRunReadsThePreRunStore, TestLuaReads_StoreErrorRaises, TestLuaReads_OwnRowIsPreStep, TestLuaReads_StepFaceAndFaces. Context cancellation is already covered by TestLuaStep_RunawayScriptInterruptedByContext.
status: addressed
---
