---
id: RR-L8J6X1I
type: review-response
title: Restricted principals make big ranges too_large
finding: For a reader with hidden fields any range over 2000 rows is too_large; a 16-way split takes several levels. Split straight to buckets of about 1500 local elements and pin a budget for it.
severity: significant
resolution: A too_large span is split by type or bucketed at local boundaries into buckets of at most 1500 rows. The budget test runs with and without a too_large threshold of 2000.
status: addressed
---
