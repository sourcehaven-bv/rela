---
id: RR-CEWI0QG
type: review-response
title: Row limit duplicated the service batch limit
finding: maxAppendRecordRows and records.MaxBatch were not pinned together.
severity: nit
resolution: Exported lua.MaxAppendRecordRows and added TestRecords_LuaRowLimitIsServiceBatch.
status: addressed
---
