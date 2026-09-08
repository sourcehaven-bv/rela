---
id: RR-E2KNIT
type: review-response
title: listpushdown differential test compared the naive comparator to itself
finding: The test runs over memstore, so both the pushed and Go paths use graphquerynaive; the cross-backend guarantee lives in storetest, whose fixture lacked colliding/null cases.
severity: minor
resolution: The adversarial cases (colliding keys, absent keys, JSON null, asc/desc, windows, count under paging) live in RunGraphPagingTests, which every backend runs through RunAll; the dataentry test keeps its role of pinning the handler's eligibility and page arithmetic.
status: addressed
---
