---
id: RR-PN7OTT
type: review-response
title: 'Key name offer: transitions collides with the transitions vocabulary and hides that it refuses posts'
finding: 'The plan names the key session.offer with the value transitions (Approach 2) because it "says what changes for a person: which types the composer offers". The same word already names _actions.transitions, read_session transitions and the decision buttons (internal/realtime/session.go:344-358, service.go:292). An operator will read offer: transitions as "offer the transition buttons". The key also does more than offer: it makes posts fail with 409 session_wrong_state (session.go:410-421) and makes triggers skip their post. A skipped change is marked seen and is never retried (trigger.go:1238-1246). The plan''s own docs row explains the key only in terms of in:. Change: name the key after the rule it applies, the in: that a type without an authored in: gets. Examples are session.default_in: keyed, or post_in: keyed_states. Keep it a closed enum with absent meaning today''s behaviour. Then update AC 1, the error text ("default_in may only be keyed"), the hint in keys.go and the docs. Decide this before release: a config key cannot be renamed later without a migration.'
severity: significant
resolution: 'Renamed the key to session.default_in with the one value keyed, a closed enum, and updated AC 1, the error text, the keys.go hint and the docs. Approach 2 records why: it names the in: it sets, covers the 409 and the skipped triggers, and follows the default_<key> naming of data-entry.yaml (default_view, default_sort).'
status: addressed
---
