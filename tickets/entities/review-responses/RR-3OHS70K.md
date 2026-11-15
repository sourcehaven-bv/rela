---
id: RR-3OHS70K
type: review-response
title: Diff must read raw rows; gate before naming a row
finding: The plan read current state through the visibility reader, so redacted properties look absent (re-import never a no-op) and hidden rows plan as creates.
severity: critical
resolution: The planner reads raw rows (store.Store ListEntities/ListRelations) as a write-prep read and gates every matched row with GetAddress before the plan names it; a key held by a row the principal cannot read is a record error (the accepted one-bit channel, as in remotesync). Writes are authorized by the manager.
status: addressed
---
