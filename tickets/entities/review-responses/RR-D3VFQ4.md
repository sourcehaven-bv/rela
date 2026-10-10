---
id: RR-D3VFQ4
type: review-response
title: Pointer drag over a keyboard preview writes the wrong delta
finding: onPointerDown took the drawn (previewed) dates as origin, so a drag after arrow keys wrote only the pointer delta.
severity: significant
resolution: 'The gesture keeps the held preview''s origin (from) and continues from the drawn window. Regression test: two arrows then a one-day drag writes +3.'
status: addressed
---
