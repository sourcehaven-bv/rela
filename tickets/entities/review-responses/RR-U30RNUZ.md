---
id: RR-U30RNUZ
type: review-response
title: No room for a durable side-effect mark after a move
finding: doc stored exactly flow.Instance so a pending-outputs mark (as channels OutputsAction) would need a column migration later.
severity: significant
resolution: 'doc is a run-level JSON envelope {instance: ...} so the runtime can add a pending mark without a migration; the mark itself is carried to TKT-9IT3LN8.'
status: addressed
---
