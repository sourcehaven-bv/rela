---
id: RR-7DUSCM
type: review-response
title: Fallback frame on a busy list subscription reveals channel changes the subscriber cannot see
finding: 'When a round runs out of room or time, liveHub.fallback marks every non-inbox subscription as changed, which sends a refetch frame (internal/dataentry/live.go:904-916). The inbox is exempt for exactly this reason: "a change it cannot see may have chosen its round" (internal/dataentry/live_inbox.go:46-49, live.go:886-890). Under the plan, a change to a run channel on a hidden ticket, or to a channel the caller may not read, selects every busy board, and an overflowing round then sends those boards a frame that their visible answer does not justify. That contradicts the Security section''s claim that the answer does not say a channel exists. In a topic round the plan should defer busy-selected list subscriptions to pending with no fallback frame, like inbox subscriptions, or select only on keys whose anchors are on the subscriber''s gated page (see the previous finding). It should add a test modelled on the live_inbox_test.go:936 no-fallback case.'
severity: significant
resolution: 'Resolved by the same move: no channel change selects a busy list subscription, so a round fallback never frames a board for a change it cannot see. The no-fallback rule is part of the follow-up''s scope.'
status: addressed
---
