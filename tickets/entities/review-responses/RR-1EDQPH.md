---
id: RR-1EDQPH
type: review-response
title: Capture-phase click handlers defeat @click.stop
finding: On kanban cards and the list title column a capture handler opens the row before the widget sees the click, so the file list cannot open.
severity: significant
resolution: 'Cards, calendar chips and the list title column pass no entityId, so the widget renders plain text there: no link, no details, no click.stop.'
status: addressed
---
