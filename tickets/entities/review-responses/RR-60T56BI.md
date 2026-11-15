---
id: RR-60T56BI
type: review-response
title: Each type's first window starts at n even when fewer rows are needed
finding: firstNByTypes started every type at a window of n.
severity: nit
resolution: The first window of a type is n minus the results already kept.
status: addressed
---
