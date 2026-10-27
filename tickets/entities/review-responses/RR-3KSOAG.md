---
id: RR-3KSOAG
type: review-response
title: Head CTE is quadratic in the hops
finding: The head step picked the last hop with a correlated max subquery over fwd per row so it was quadratic in the hop count of one tag.
severity: significant
resolution: head now takes row_number() over the fwd rows of one tagged vseq ordered by hop descending. Works on PostgreSQL and SQLite.
status: addressed
---
