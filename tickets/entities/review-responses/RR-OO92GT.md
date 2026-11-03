---
id: RR-OO92GT
type: review-response
title: Unknown currency card property reported twice
finding: The card field loop and checkIntegerProperty both reported a property missing from the entity.
severity: nit
resolution: checkIntegerProperty takes reportUnknown; card fields pass false. TestKanbanAmounts_UnknownCardPropertyReportedOnce.
status: addressed
---
