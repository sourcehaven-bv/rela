---
id: RR-XFEMIM
type: review-response
title: Undeclared property with several values shows only the first
finding: Without a PropertyDef raw was values[0], dropping the rest for an undeclared or not-yet-loaded property.
severity: minor
resolution: 'ViewTableCell passes the full array when there is more than one value. Test: undeclared property shows all values joined.'
status: addressed
---
