---
id: RR-1WA5X2
type: review-response
title: Inbox link test covers mention rows only
finding: frontend/src/views/InboxView.test.ts:155 is named for mentions, and AC 9 says mention and reply rows. Both come from the same mentions list (InboxView.vue:104-110), so the code is right, but a reply-kind row in the fixture would pin the AC as written.
severity: nit
resolution: ADDRESSED test renamed "mention and reply rows link to their seq" with a reply-kind row in the fixture.
status: addressed
---
