---
id: RR-63L9SOA
type: review-response
title: QueryConditions test misses the deny path
finding: TestQueryConditions_Lower only covered an allow-all world.
severity: minor
resolution: 'TestQueryConditions_LowerDeniedTraversal uses a policy without ticket read: the positive traversal empties the result and the negated one drops out.'
status: addressed
---
