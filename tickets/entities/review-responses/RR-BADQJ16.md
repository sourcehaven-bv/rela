---
id: RR-BADQJ16
type: review-response
title: A failed create output is not retried
finding: A create output that failed after its claim was counted as done, so a retry skipped it and the entity was never created.
severity: significant
resolution: A failed create puts the claim back (mark i), so the next round retries it. A KeptEntityError counts as done to avoid a duplicate. TestDue_FailedCreateIsRetried pins it.
status: addressed
---
