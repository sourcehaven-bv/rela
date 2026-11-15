---
id: RR-AYR44ED
type: review-response
title: Empty XML element was a record error
finding: <title/> read as a structure, not a value.
severity: significant
resolution: An element without child elements has a text, empty for <title/>, so it reads as absent.
status: addressed
---
