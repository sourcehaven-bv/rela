---
id: RR-E57RFMM
type: review-response
title: File store last_seq on inbox entry may differ from the head's
finding: The entry takes LastSeq from meta.yaml via ListWaiting; Head reads it from the log. After a crash between segment append and meta write they disagree, so the newer-messages count can be off.
severity: minor
resolution: Not changed.
reason: Every file-store listing (List, unread counts, the inbox) takes last_seq from meta.yaml, and the next write recomputes it from the log. The value only labels the newer-messages count; the open channel reads the head. Reading each segment tail per listing would break the read budget.
status: wont-fix
---
