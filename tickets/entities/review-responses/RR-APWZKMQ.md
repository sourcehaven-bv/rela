---
id: RR-APWZKMQ
type: review-response
title: Digits-only test lacks a leading-space id
finding: Sscanf skipped leading spaces, so FEAT- 5 used to count; the storetest case does not seed it.
severity: nit
reason: entity.ValidateID refuses spaces and signs, so no store can hold such an id; the comment was reworded to describe what the test pins.
status: wont-fix
---
