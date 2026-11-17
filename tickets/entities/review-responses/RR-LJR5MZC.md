---
id: RR-LJR5MZC
type: review-response
title: Fold a Where that denied hops make constant
finding: A Where whose denied traversals all read false still runs a query that cannot match, and the ticket says such a read is empty.
severity: minor
resolution: store.Cond.Constant folds empty ORs and ANDs. lowerQueryCondition and gatePageFilter report empty for a false tree and drop a true one.
status: addressed
---
