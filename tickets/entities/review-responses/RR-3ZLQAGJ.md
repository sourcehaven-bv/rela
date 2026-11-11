---
id: RR-3ZLQAGJ
type: review-response
title: A stopped push drops the uncertain holds of rows it did not reach
finding: forgetConflicts deleted address-less entries of rows missing from pushSeen even when the push ended early (401, 429/503, cancel) or the share check failed, so the next run could POST those rows again.
severity: critical
resolution: pushSeen is set only when the push finished; rows whose share check failed are marked seen. TestPush_StoppedPushKeepsHolds.
status: addressed
---
