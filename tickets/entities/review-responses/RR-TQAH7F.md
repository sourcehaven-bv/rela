---
id: RR-TQAH7F
type: review-response
title: String-matching for missing-peer classification is fragile; use errors.Is sentinels
finding: isMissingPeerCondition/isSoftCondition branch on strings.Contains(err.Error(), 'target entity not found') etc., while entitymanager already exports typed sentinels (ErrEntityNotFound, ErrRelationNotFound). A reword of the manager's Errorf would silently reclassify a dangling peer as a hard 500. The code comment already admits it's fragile; the fix now leans on it more heavily.
severity: minor
reason: Pre-existing fragility that predates this fix; works correctly today (strings match exactly). Deferred to a follow-up that switches to errors.Is(err, entitymanager.ErrEntityNotFound) for robustness. Non-blocking (minor), out of scope for the security fix.
status: deferred
---
