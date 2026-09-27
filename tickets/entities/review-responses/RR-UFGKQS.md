---
id: RR-UFGKQS
type: review-response
title: Unprimed rows rebind on every verdict call
finding: traversalFor answered live but did not store the answer in an existing memo, so repeated verdicts on one row repeated the store query.
severity: significant
resolution: traversalFor stores live answers in the memo and PrimeTraversals skips memoized rows; TestQueryBudget_GetEntityACLRelatedWhenBindsOnce.
status: addressed
---
