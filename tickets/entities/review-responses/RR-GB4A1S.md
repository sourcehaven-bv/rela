---
id: RR-GB4A1S
type: review-response
title: 'Generation lock can stick: a 201 does not always change the generation'
finding: 'Approach 9 ''Why the generation lock is safe'' claims a 201 means the session moved or someone else moved it. False when the client''s head or descriptor is stale and nobody else moves the session: (a) a realtime.yaml reload removed the type from the state''s on: (no in:, so the post is stored and moves nothing); (b) the caller lost the transition role but kept post (e2e fixture: post [member, lead], roles [lead]; an acl change demotes alice); (c) ch.refetch() fails after the 201 (network), so the head never reaches gen+1 even though the server moved. In all three shownDecisions stays [] while generation === decidedGen, which hides every other still-valid decision (for example Revise) until some later move. Release the lock on evidence, not only on a generation change: clear decidedGen when a head fetched after the 201 lands (watch ch.head identity together with _actions), and on refetch failure. Add a ChannelsPanel test: 201, follow-up head with the same generation and transitions still listing the other type, decisions come back.'
severity: significant
resolution: Replaced the generation lock with a per-click lock released in finally (POST plus an unfolded refreshHead, or either failing), and decisions now show only when the held _actions were fetched at the head's current generation (useChannel.actionsGeneration set in takeHead), with a test for a 201 that moves nothing.
status: addressed
---
