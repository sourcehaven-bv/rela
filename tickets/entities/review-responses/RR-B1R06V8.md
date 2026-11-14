---
id: RR-B1R06V8
type: review-response
title: Unique lookup missed the scalar spelling
finding: 'The unique: PropEqual lookup did not set Scalar, so pgstore used containment and not the derived unique index.'
severity: significant
resolution: 'Set Scalar: true and read only the writer''s face, which is all compareProps judges.'
status: addressed
---
