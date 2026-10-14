---
id: RR-FMHWYV
type: review-response
title: RFC 822 zones UT/Z never parse; unknown zones get offset 0
finding: time.Parse rejects short zone names and gives unknown names a zero offset.
severity: minor
resolution: numericZone rewrites known names to numeric offsets; unknown names yield an unreadable date (raw kept). Tests for UT, Z, lowercase est, CEST.
status: addressed
---
