---
id: RR-JU3ZFB
type: review-response
title: Fallback to the pre-read family was stale
finding: When res.DeletedEntities was empty the audit and version capture fell back to the family read before the Tx, which could name faces the store never removed.
severity: minor
resolution: Removed the fallback and its coverage-ignore. Every backend reports the rows it removed; capture and audit use only res.DeletedEntities.
status: addressed
---
