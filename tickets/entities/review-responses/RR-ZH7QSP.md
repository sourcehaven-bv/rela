---
id: RR-ZH7QSP
type: review-response
title: Segment ending in a backslash prints a literal closing tag
finding: A covered block ending in an escaping backslash made the inserted </mark> render as text.
severity: significant
resolution: quotefind keeps segment edges off an escaping backslash (end pulled back, start moved onto it). Segment tests and the golden fixture cover it.
status: addressed
---
