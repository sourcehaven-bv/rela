---
id: RR-8WT2EL
type: review-response
title: Store failures answered 404 with internal detail
finding: The default branch returned relation_not_found with err.Error(), which could name a hidden sibling's edge key.
severity: minor
resolution: ErrRelationNotFound maps to 404 without detail; any other error is logged and answered 500 internal_error without detail. The write error no longer names the edge.
status: addressed
---
