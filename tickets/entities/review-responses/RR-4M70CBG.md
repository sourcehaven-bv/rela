---
id: RR-4M70CBG
type: review-response
title: LIMIT literal couples page size to the statement cache
finding: The page LIMIT is part of the SQL text, so a page sized to its batch would add one cached statement per batch size.
severity: minor
resolution: The page grows only for a one-shot statement, which is never cached. pagePlan documents why.
status: addressed
---
