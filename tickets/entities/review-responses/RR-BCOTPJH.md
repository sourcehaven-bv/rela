---
id: RR-BCOTPJH
type: review-response
title: Own-token history overwrites another user's revert
finding: '[security] Tokens hash the value, so another user setting a field back to a value this client held earlier reads as our own: the stale rule hides it and the next 412 resends ours, silently destroying their change (e.g. a checkbox ticked on and off by us, on again by them).'
severity: critical
resolution: 'Removed the own-token rule and the stale-snapshot rule: a 412 always takes the merge or conflict path. Hashes cannot order versions, and the narrowed form (current == newest) can never hold on a 412. Late reads are kept out by the session''s read order. Three revert tests (body 412, body snapshot, property 412) fail on the previous code.'
status: addressed
---
