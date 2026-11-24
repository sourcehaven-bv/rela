---
id: RR-N8F2KZG
type: review-response
title: Large offsets produce a malformed ISO year
finding: Large offsets produce a malformed ISO year
severity: significant
resolution: iso() returns null outside the years 1 to 9999; over 9999 years is tested.
status: addressed
---
