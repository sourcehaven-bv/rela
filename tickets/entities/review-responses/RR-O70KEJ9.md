---
id: RR-O70KEJ9
type: review-response
title: Absent value passed under not
finding: An over-cap or withheld leaf read as false or unset, so a negated condition kept the row.
severity: significant
resolution: Absent values drop the row for every comparison polarity on all backends; tested.
status: addressed
---
