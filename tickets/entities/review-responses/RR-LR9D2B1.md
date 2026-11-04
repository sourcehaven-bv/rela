---
id: RR-LR9D2B1
type: review-response
title: Form-condition lint types decimal as number
finding: conditionlint maps decimal to NumberType while predicatefns uses DecimalType; form.price == '12.50' fails config load.
severity: minor
reason: Deferred to a follow-up decimal ticket; the lint is stricter than the engine (it refuses a valid condition) and never admits a wrong one; aligning it needs its own lint tests.
status: deferred
---
