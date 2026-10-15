---
id: RR-F50TE4
type: review-response
title: formatAmount builds a formatter per call
finding: A new Intl.NumberFormat per card per render is costly on large boards.
severity: nit
resolution: Formatters are cached per locale, currency and digits.
status: addressed
---
