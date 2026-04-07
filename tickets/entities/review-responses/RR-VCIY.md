---
id: RR-VCIY
type: review-response
title: TrimLeft only handles spaces, not tabs
finding: extractListItems uses TrimLeft(s, " ") which only handles ASCII spaces. Tabs or non-breaking spaces from goldmark's checkbox handling would leak whitespace into task item text.
severity: significant
resolution: Changed TrimLeft(s, " ") to TrimLeft(s, " \t") to handle tab characters. Inlined into the per-block extraction since we now only process the first text block.
status: addressed
---
