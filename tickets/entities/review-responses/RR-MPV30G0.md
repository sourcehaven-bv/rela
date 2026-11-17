---
id: RR-MPV30G0
type: review-response
title: 'Design: property rename leaves stale placeholders in triggers'
finding: references.ts rewrote only on.property and set keys. {{new.old}} placeholders in values and where/when clauses keep the old name and silently write empty strings.
severity: significant
resolution: renameTriggerReferences in the SPA renames on.property and when and where filters and set keys and placeholders in output texts and check.where. Locked triggers are skipped. Test in configDraft.test.ts.
status: addressed
---
