---
id: RR-KC2U
type: review-response
title: Double-click protection / idempotency
finding: Rapid double-clicks will create duplicate entities (e.g. two daily notes). Frontend must disable button during inflight. Document that scripts should be idempotent.
severity: significant
resolution: Disabled attribute during in-flight prevents double-click. Scripts documented as 'should be idempotent' in user docs.
status: addressed
---
