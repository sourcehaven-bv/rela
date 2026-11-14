---
id: RR-DEEC72H
type: review-response
title: Identity stamping is bundled with lowering
finding: ConditionLowerer carries both StampQueryIdentity and Lower.
severity: minor
reason: current_user must bind to the same caller in the store and in Go. The one consumer that lowers is the one that needs the stamp, so the pair stays together. Documented on QueryConditions.
status: wont-fix
---
