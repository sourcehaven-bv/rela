---
id: RR-WB5YXZ
type: review-response
title: A condition that reads no entity property is always refused
finding: 'internal/appbuild/realtime_service.go:199-208 binds `AnchorView.Record` only when `len(props) > 0`, and internal/realtime/authz.go:293 (`conditionHolds`) returns false on a nil Record. A valid `condition: "today() >= date_add(...)"` on a type with no entity-property roles therefore denies every caller, in both the affordance and the POST, with the misleading "does not meet the condition" title. Bind the record whenever the op has conditions (or always; it costs nothing), and give the fake in helpers_test.go:248 the same rule plus a test row.'
severity: significant
resolution: The anchor reader in appbuild and the realtime fake now bind Record whenever the schema defines the type, with or without requested properties. The AnchorView doc says so. New config type `timed` (close condition `days_between(today(), today()) == 0`) and TestLifecycle_ConditionReadingNoProperty pin it. That test fails with the old fake rule.
status: addressed
---
