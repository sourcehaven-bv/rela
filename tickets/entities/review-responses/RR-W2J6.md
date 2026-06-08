---
id: RR-W2J6
type: review-response
title: TKT-VMD8 AC4 ignores read-deny/write-grant role combination — useless but reachable
finding: '_actions.create == false under DenyAll falls out naturally only IF the policy load rejects ''read:[] + write:[ticket]'' roles. It doesn''t today: translateVerb(''create'', ''ticket'', '''') is evaluated by AuthorizeWrite which doesn''t consult ReadQuery. A misconfigured policy with read:[] + write:[ticket] produces data:[] + _actions.create:true — principal can POST a ticket they then can''t see. Useless from UX standpoint but reachable. Pick one and pin in AC4: (a) Policy loader rejects roles where write contains types not in read at startup (preferred — config-time check, no runtime cost); (b) computeCollectionActions intersects with ReadQuery(type).DenyAll and zeroes create when DenyAll (runtime cost per request). (a) is right; document in security guide, add a policy-load test.'
severity: significant
reason: 'The read-deny/write-grant policy-load check is a list-side concern: it''s computeCollectionActions on the list response that surfaces _actions.create:true while data:[] is empty. This PR doesn''t change computeCollectionActions or add the suggested startup-time policy-load check. The right fix is option (a) from the RR — config-time policy validation rejecting roles where write contains types absent from read. Deferring to TKT-VMD8 because that ticket already touches the list-collection code path AND because the startup validation is a one-place addition that naturally pairs with the list-side AC4 work.'
status: deferred
---
