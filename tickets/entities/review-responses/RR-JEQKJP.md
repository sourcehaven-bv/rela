---
id: RR-JEQKJP
type: review-response
title: Segment edges include inline markup and code
finding: Segments built from inline content lines can split emphasis (<mark>**Head</mark>ing**); client overlapsCode drops any segment touching inline code.
severity: significant
resolution: 'Plan revised: segments clamped to first/last ast.Text node of each block and split around CodeSpan; client code check kept as per-segment backstop.'
status: addressed
---
