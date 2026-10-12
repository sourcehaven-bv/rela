---
id: RR-ZW0I2E
type: review-response
title: Renew trusts lease_id alone and never re-checks access
finding: 'Security design review: renew succeeded after read access was revoked or the entity was deleted/recreated, leaking existence and keeping a badge on a recreated entity.'
severity: significant
resolution: 'Plan: renew re-runs the claim checks and on failure drops the lease and returns ErrLost; release answers the same whether or not the lease existed; AC3 covers revocation, delete and recreate.'
status: addressed
---
