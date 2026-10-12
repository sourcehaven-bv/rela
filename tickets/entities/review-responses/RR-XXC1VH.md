---
id: RR-XXC1VH
type: review-response
title: Round commit holds h.mu across the loop undocumented
finding: S1 needs the lock; document the cost so it is not optimised back to per-conn locking
severity: minor
resolution: 'Fixed in the review-fix commit (Presence: review fixes) with tests; presence tests pass 50 of 50 runs under race.'
status: addressed
---
