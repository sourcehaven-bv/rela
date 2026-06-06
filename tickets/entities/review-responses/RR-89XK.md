---
id: RR-89XK
type: review-response
title: GraphCount error → 500 is too coarse — distinguish context cancellation / deadline
finding: 'Plan says ''GraphCount error → 500 acl_query_failed; do NOT silently allow.'' Correct for fail-closed, but the error is overloaded. GraphCount can return: (a) pgx connection-closed, planner errors, real DB failures — these are 500s; (b) context.Canceled (client disconnected) — should NOT be 500, emit nothing; (c) context.DeadlineExceeded — 504, not 500. A high-cardinality 500 alert on client disconnects is paging noise. Pin: errors.Is(err, context.Canceled) → emit nothing (client gone); errors.Is(err, context.DeadlineExceeded) → 504; else 500 acl_query_failed.'
severity: significant
status: open
---
