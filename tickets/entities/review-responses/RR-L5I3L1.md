---
id: RR-L5I3L1
type: review-response
title: 'list: true erased the type''s display formatter in cells'
finding: 'densePropertyRoutingHint checked propertyDef.list before the type switch (mirroring defaultWidgetFor, which is the EDIT-side dispatcher). On the display side this erased the type''s formatter: a date + list:true column rendered ISO strings as grey enum badges, and a list-valued rrule rendered an em-dash (the value vanished entirely, because MultiSelectWidget does not unwrap a single-element array the way formatCellValue does). Both are regressions against the string path being replaced.'
severity: critical
resolution: Moved the list check after the enum branch but before the type switch, routing every non-enum list to preformatted text so formatCellValue reproduces the pre-migration string byte for byte. Enum lists still route to enum-list and badge. Pinned by an it.each over all five list-valued types plus end-to-end render tests for the date and rrule cases.
status: addressed
---
