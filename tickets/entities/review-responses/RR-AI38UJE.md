---
id: RR-AI38UJE
type: review-response
title: flow imports realtimelimits for ReminderSlack
finding: flow depends on realtimelimits only for ReminderSlack; a generic engine should own its limits.
severity: minor
reason: M1 defines the flow limits when the run store lands; moving the constant now would churn realtimelimits for one value.
status: deferred
---
