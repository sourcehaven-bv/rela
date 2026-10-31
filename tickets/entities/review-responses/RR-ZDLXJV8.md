---
id: RR-ZDLXJV8
type: review-response
title: useReview and useDiscardDraft tests import the store through the alias
finding: useDiscardDraft.test.ts and useReview.test.ts import @/configure/configDraft while their neighbouring imports are relative.
severity: nit
resolution: Both now import ../configDraft.
status: addressed
---
