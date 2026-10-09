---
id: RR-PIA9DZ
type: review-response
title: Incoming movable check reads each source row
finding: Deciding movable on an incoming list needs the meta affordance and the ACL update for every distinct source. Reading each source row separately is a per-row lookup on a collection read (CLAUDE.md collection rule). Batch the source rows as linkablePage does and pin the cost with a storetest.Counting test at 10 and 50 rows.
severity: significant
resolution: incomingSources reads every source of the list through batchedSources (extracted from linkablePage) in one batch; TestQueryBudget_IncomingOrderedListIsSizeIndependent pins 10 reads at 10 and 50 rows.
status: addressed
---
