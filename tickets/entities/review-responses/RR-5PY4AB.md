---
id: RR-5PY4AB
type: review-response
title: Delete-then-restore still skips legality of the move from the deleted state
finding: Restoring a deleted row to an earlier version is a backward move no edge declares; UpdateEntity would refuse it on a live row. Neither the ticket nor the docs said this was accepted.
severity: significant
resolution: 'Documented as an accepted residual in the EnforceRestore godoc, internal/entitymanager/CLAUDE.md and the BUG-KK1UXH decision: the owner ruled a restore brings back the chosen version''s state, and the trust boundary is the right to delete the entity and read its deleted history.'
status: addressed
---
