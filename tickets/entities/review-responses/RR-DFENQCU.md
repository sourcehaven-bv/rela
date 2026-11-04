---
id: RR-DFENQCU
type: review-response
title: Small implementation details
finding: Memo by joined value was ambiguous; the inbox stub DecisionComposer would need a no-op leaveDecision.
severity: nit
resolution: Memo uses JSON.stringify; leaveDecision removed, so the inbox stub needs no change.
status: addressed
---
