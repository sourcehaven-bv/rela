---
id: RR-MGG2SS
type: review-response
title: Native min/max blocks form submit
finding: Browser constraint validation would refuse to submit a form with an out-of-range number, making the soft warning a hard refusal and locking out saves of entities that already hold one.
severity: significant
resolution: NumberWidget sets min/max only while the value is empty or in range, so the stepper is bounded but validity never fails. Test asserts validity on an out-of-range value.
status: addressed
---
