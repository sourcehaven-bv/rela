---
id: RR-DC5KM3
type: review-response
title: Source reload fault yields a short list
finding: A ListEntityHeaders or read-gate fault in loadViewEntities was logged and swallowed, so a backlinks section showed a short or empty list while the built-in panel was hidden.
severity: significant
resolution: loadViewEntities and gateLoadedEntities now return the first fault beside the partial result; collectBacklinks wraps it in gateFaultError. Relation steps keep the partial collection as before. TestViewMentions_FaultFailsTheView/source_reload.
status: addressed
---
