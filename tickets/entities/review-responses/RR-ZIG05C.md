---
id: RR-ZIG05C
type: review-response
title: Unreported fields move the base
finding: baseDiffers is set before the reported check, so a local edit to a field theirs did not report sets Retag and is later overwritten without being pushed.
severity: critical
resolution: 'syncmerge counts a field as changed since base only when both sides report it; Result.Complete (Lua: complete) gates the retag; guide and syncloop test tag only on a complete conflict-free report. Tests: unreported-local-edit subtest, TestConvergence with dropped fields, TestSyncMerge_ResultTable.'
status: addressed
---
