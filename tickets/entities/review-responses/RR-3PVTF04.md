---
id: RR-3PVTF04
type: review-response
title: .. returned nodes twice
finding: Chained .. steps yielded nested matches more than once and multiplied work.
severity: significant
resolution: descend keeps one visited set per step, so each node is searched and yielded once.
status: addressed
---
