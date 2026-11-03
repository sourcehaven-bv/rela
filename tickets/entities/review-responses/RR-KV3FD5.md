---
id: RR-KV3FD5
type: review-response
title: Managed order on tolerated creates untested
finding: The change applies managed order and templates to tolerated bounded creates, but no test checked it.
severity: minor
resolution: TestReplaceRelations_TolerateTypeMismatch now uses an orderable bounded relation and asserts the created edge has _order_out.
status: addressed
---
