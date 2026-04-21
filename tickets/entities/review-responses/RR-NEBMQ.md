---
id: RR-NEBMQ
type: review-response
title: Reconciler doesn't early-exit on empty desired
finding: Empty-map desired skips the nil check and performs a full outgoingRelations read for nothing. Add `if len(desired) == 0 { return nil }` — also the common case after the frontend filtered out card-managed relations and no chip-picker relations remain.
severity: nit
resolution: Early-return is now `if len(desired) == 0 { return nil }` — covers both the nil and empty-map cases, which is the common path after the frontend filters out card-managed relations.
status: addressed
---
