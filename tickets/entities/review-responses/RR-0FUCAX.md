---
id: RR-0FUCAX
type: review-response
title: Alias expansion runs before the yaml alias guard
finding: checkKeys follows aliases unbounded before the strict decode
severity: minor
resolution: checkKeys stops after 100000 nodes; billion-laughs test added.
status: addressed
---
