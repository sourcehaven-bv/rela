---
id: RR-ISVXQH8
type: review-response
title: 'Design: zero ElementRange selected everything'
finding: With Hi empty meaning unbounded the zero value and Lo == Hi == empty selected every row.
severity: minor
resolution: 'Plan changed: explicit Unbounded flag; the zero value is the empty range. Validation also refuses AllowAll with Query.'
status: addressed
---
