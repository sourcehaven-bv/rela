---
id: RR-X3NDTPT
type: review-response
title: list_entities order changes without notice
finding: The order moved from natural sort to byte order by (id, face).
severity: significant
resolution: The tool description now states the order (byte-wise, REQ-10 before REQ-2); the tools/list golden is regenerated and the PR body notes it.
status: addressed
---
