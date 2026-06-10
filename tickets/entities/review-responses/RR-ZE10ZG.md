---
id: RR-ZE10ZG
type: review-response
title: WithValidation self-test missed append-ordering contract
finding: Single-rule test didn't exercise the order-preserving accumulation that ticketMeta relies on.
severity: nit
resolution: Test extended to two rules asserting Validations[0]/[1] order.
status: addressed
---
