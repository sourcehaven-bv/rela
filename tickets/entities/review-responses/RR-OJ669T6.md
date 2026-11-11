---
id: RR-OJ669T6
type: review-response
title: 'Design: claim retries under sibling activity'
finding: Claim gives up after four conflicts and reports ErrClaimed, which is wrong when siblings caused the conflicts.
severity: minor
resolution: 'Plan updated: Claim uses the shared retry helper, which only fails when its own branch changed; exhaustion returns a retry-later error, not ErrClaimed.'
status: addressed
---
