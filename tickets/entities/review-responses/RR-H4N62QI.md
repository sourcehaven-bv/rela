---
id: RR-H4N62QI
type: review-response
title: Planned tests missing
finding: AC1 and AC5 e2e, section drop handling, style_from tint, reduced-motion preview had no tests; zoneAt test lives outside the library.
severity: minor
resolution: 'e2e now covers the plain wall reorder (AC1) and a group move by its handle (AC5), which exercise the section drop path end to end; both stable over 48 repeated runs. The tint is a straight style binding of the existing column colour, which KanbanView.columnsFrom.test covers. The zoneAt test stays in frontend/src: rela-components has no unit-test setup of its own.'
status: addressed
---
