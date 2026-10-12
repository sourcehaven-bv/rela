---
id: RR-IWI1YX
type: review-response
title: Live change lost while a subscription takes its baseline
finding: 'A lease release landing inside a subscription PUT''s baseline replay is dropped: the round skips the syncing subscription but clears the change flag. Found by the presence e2e claim-badge test (about 1 in 15 under load).'
severity: significant
resolution: Phase-1 bug (entity writes lost too). A round that skips a syncing subscription marks it; the stored baseline queues a replay on the next tick; skipped over-cap pairs queue the same way. Fixed on the live-views base and propagated through the stack; tests fail without the fix; presence e2e 10 of 10.
status: addressed
---
