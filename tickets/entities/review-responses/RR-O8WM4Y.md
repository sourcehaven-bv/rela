---
id: RR-O8WM4Y
type: review-response
title: Inheritance closure cost in SQLite
finding: The correlated probe into a materialized recursive CTE is the ACL role-inheritance hot path.
severity: minor
resolution: Written as EXISTS over the closure joined on root = e.id; an EXPLAIN/budget test covers the EntityInheritThrough shape.
status: addressed
---
