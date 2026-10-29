---
id: RR-OPSAK9
type: review-response
title: Branching decisions lose expect_generation in the SPA, which removes the protection the motivating plan-review case needs
finding: 'decisionViews opens the form for any type with several targets or a guarded edge (frontend/src/components/entity/channels/channelView.ts:395-401). openForm only selects the type in the composer (useSessionDecisions.ts:213-226). The composer send then posts without a generation (OpenChannel.vue:306-321, `postTyped(event.type, converted.props, event.replyTo)`). So every branching decision goes out without expect_generation. Today the PoC''s `approve` is a one-click button and sends it. After a move to a `decide` enum type, a stale Approve meant for an earlier plan-review can land on a re-entered plan-review and move it. Re-entry is the case docs/realtime.md:550-566 introduces expect_generation for. Fix in scope: when a decision opens the form, capture the head generation at the click and pass it as expectGeneration on that form''s Send (and clear it when the user changes type or the decision becomes stale). Add a useSessionDecisions.test.ts case. Or record this explicitly as a blocker for the PoC moving to branching and file the follow-up now.'
severity: significant
resolution: 'Added AC 13 and SPA step: `openForm` captures the head generation, the composer Send posts it as `expect_generation`, and it is cleared on type change, session move, send and reset, with useSessionDecisions.test.ts cases.'
status: addressed
---
