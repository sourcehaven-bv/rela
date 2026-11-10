---
id: RR-S9NA7S0
type: review-response
title: Test gaps in scoped updates
finding: No test for guarded transitions or for cascades running gated.
severity: minor
resolution: TestScopedUpdates_Transitions covers legal and refused moves and the guard. Cascades use the existing elevated() handle and are pinned by TestManager_Elevated_DoesNotLeakIntoNestedCascade.
status: addressed
---
