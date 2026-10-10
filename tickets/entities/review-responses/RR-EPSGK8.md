---
id: RR-EPSGK8
type: review-response
title: Card comment count went stale after commenting from the board
finding: Comment writes raise no store event and did not invalidate the list query, so a card kept its old count after a comment was added in the side panel.
severity: significant
resolution: Fixed. EntityDetail handles comment 'changed' with onCommentsChanged, which invalidates entityKeys.list(type) and reloads the thread. Pinned by a test in EntityDetail.accept.test.ts.
status: addressed
---
