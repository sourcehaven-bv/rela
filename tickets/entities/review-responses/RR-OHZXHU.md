---
id: RR-OHZXHU
type: review-response
title: Cached next link can hide new to-dos
finding: A 304 on a full last page reused a nil next link, so a new page was never read.
severity: significant
resolution: The last page of a listing is never fetched conditionally.
status: addressed
---
