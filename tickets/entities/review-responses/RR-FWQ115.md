---
id: RR-FWQ115
type: review-response
title: Post-commit failure reported as failed conversion
finding: When finishCreate fails after commit, or the relation write fails after the entity write on fs/mem, Convert returns an error with no id. The editor leaves the item and a retry creates a duplicate entity.
severity: significant
resolution: A failure after the commit is now TaskItemResult.FollowUpErr. The handler logs it and answers 200 with a follow_up_failed warning, so the editor still updates the item. A failure after the entity write on a store that cannot roll back is a KeptEntityError, answered as 500 conversion_incomplete naming the created id. Pinned by handler tests.
status: addressed
---
