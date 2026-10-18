---
id: RR-WIO22D
type: review-response
title: Small signature and test mismatches
finding: CreateEntity takes *entity.Entity, not a value. In AC6 the losing concurrent op may answer lifecycle_wrong_state or conflict depending on timing, so the test must accept either. T4b and T4c both edit channels_handler.go, channels_errors.go, wire types and openapi, so "review in parallel" will produce merge conflicts.
severity: nit
resolution: CreateEntity takes *entity.Entity. The AC6 test accepts either lifecycle_wrong_state or conflict for the loser. T4b and T4c are stacked in sequence, not reviewed in parallel.
status: addressed
---
