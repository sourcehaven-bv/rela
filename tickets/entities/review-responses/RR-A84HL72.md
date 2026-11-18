---
id: RR-A84HL72
type: review-response
title: Verify fails spuriously when a prune races it
finding: A prune committing during Verify moved the anchor and Verify reported a break.
severity: minor
resolution: Verify restarts up to three times when the anchor moved during the walk.
status: addressed
---
