---
id: RR-RMUMM4
type: review-response
title: Presence tests rely on short quiet windows
finding: presenceSettle is shorter than one tick so negative checks pass without a round; make rounds deterministic
severity: minor
resolution: 'Fixed in the review-fix commit (Presence: review fixes) with tests; presence tests pass 50 of 50 runs under race.'
status: addressed
---
