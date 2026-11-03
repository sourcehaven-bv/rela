---
id: RR-YGEDLA
type: review-response
title: The transitions and inbox assertions are vacuous for in:, and the docs claim they "follow in:"
finding: 'fireable() only lists types keyed by on: in the current state, and the new load check (message_in.go:79-86) forces every such state into the type''s in:. So in: can never remove a transition; the only case where it could is a stranded session, which already has no transitions. TestAffordances_MessageInInbox (session_affordance_test.go:675) and the transitions/read_session columns of TestAffordances_MessageIn would pass identically without in:, so they do not test AC 3 for those surfaces. docs/realtime.md:592 (and the GUIDE copy) lists "the session''s transitions ... read_session and the inbox" as following in:, which suggests a filter that cannot happen. Fix: say in the test docs that these are regression pins and not proof of the filter. In the docs, say that in: narrows actions.post and can_post, and that transitions are already limited to the state, so the load check keeps them consistent.'
severity: minor
resolution: 'The test docs call the transitions, read_session and inbox checks regression pins, and the docs now say in: narrows actions.post and can_post while transitions are already limited by on: and kept consistent by the load check.'
status: addressed
---
