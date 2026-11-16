---
id: RR-MFE44NV
type: review-response
title: Closed period can race an in-flight posting
finding: The lock date is read before the append transaction opens, so a period closed while a posting is in flight can still receive it.
severity: significant
resolution: Append and Reverse run the owner check, the lock-date read and the insert inside the entity store's transaction on postgres and sqlite, so a period close is ordered against postings. TestRecords_ClosingAPeriodIsOrderedAgainstAppends.
reason: 'Holds only for the file and memory backends, documented in GUIDE-records: there the lock-date read and the insert are not locked together, so a posting in flight while the period closes can still land.'
status: addressed
---
