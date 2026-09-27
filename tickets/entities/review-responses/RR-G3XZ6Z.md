---
id: RR-G3XZ6Z
type: review-response
title: Automation related() on create is constant
finding: The entity is persisted before its relations; so on a created trigger related() is always false and not related() always true.
severity: significant
resolution: NewEngineFromMetamodel refuses related() in a condition on a created trigger (test case in TestNewEngine_RelatedRefusedAtLoad). Update-trigger timing documented.
status: addressed
---
