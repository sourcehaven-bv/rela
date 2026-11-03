---
id: RR-BXU04F
type: review-response
title: useMarkRead case does not exercise hidden
finding: 'frontend/src/composables/useMarkRead.test.ts:112: useMarkRead takes no presentation input, so a message of type ''claim'' is just a string here and the case is the same as any newest-message case. It cannot catch a later "mark up to the newest shown row", because that change would add a new input and rewrite the test. Accepted as a marker of intent, but the comment should not claim it guards the behavior. A stronger pin would be in an OpenChannel test with a channels: block that hides the newest type.'
severity: nit
resolution: ADDRESSED comment now says the case marks intent and cannot catch a mark-up-to-newest-shown change.
status: addressed
---
