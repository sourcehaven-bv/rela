---
id: RR-C11LL8
type: review-response
title: Compile-time companion for the GraphQuery zero-value gate
finding: A new GraphQuery field silently sends sqlite queries to the slow path; a reflection test over the field set would turn that into a failing test naming the fix.
severity: nit
reason: The safe direction already holds. The gate test lists every current non-simple field explicitly, so a reviewer adding a field sees where to extend it. A field-set snapshot test is worth adding with the SQL relation-predicate work for sqlite.
status: deferred
---
