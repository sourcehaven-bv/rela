---
id: RR-MFE44NV
type: review-response
title: Closed period can race an in-flight posting
finding: The lock date is read before the append transaction opens, so a period closed while a posting is in flight can still receive it.
severity: significant
resolution: Append and Reverse run the owner check, the lock-date read and the insert inside the entity store's transaction on postgres and sqlite, so a period close is ordered against postings. TestRecords_ClosingAPeriodIsOrderedAgainstAppends.
reason: 'Documented as best-effort in GUIDE-records: a posting that started before the period was closed can still land. Reading the lock date inside the append transaction is tracked in TKT-EV8U8T4.'
status: addressed
---
