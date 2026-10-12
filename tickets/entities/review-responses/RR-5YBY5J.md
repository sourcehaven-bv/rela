---
id: RR-5YBY5J
type: review-response
title: Subs that got 200 without joining never get presence rounds
finding: cap-refused and Unknown subs have no joined face so the presence-round filter skips them and their viewer list goes stale; capped subs never retry
severity: significant
resolution: 'Fixed in the review-fix commit (Presence: review fixes) with tests; presence tests pass 50 of 50 runs under race.'
status: addressed
---
