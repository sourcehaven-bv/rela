---
id: RR-PAPZI8X
type: review-response
title: Gantt postgres test reuses the nodes map across sizes
finding: nodes was declared outside the loop, so the n=1200 case started with the n=100 nodes.
severity: nit
resolution: nodes is declared inside the loop.
status: addressed
---
