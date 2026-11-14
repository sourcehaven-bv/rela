---
id: RR-BNCX2L2
type: review-response
title: IDs pushdown decline is wider than the graph branch
finding: composeReadScope declined pushdown for every query with IDs. Only the graph branch drops the ids; the allow branch keeps them on its EntityQuery.
severity: minor
resolution: The IDs check now runs after the allow and deny branches just before the graph branch. TestListPushdown_DeclinesQueryWithIDs also asserts an allow-all query with ids still pushes down.
status: addressed
---
