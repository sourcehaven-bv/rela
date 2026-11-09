---
id: RR-CI3NWY1
type: review-response
title: Long checkbox labels wrap below the checkbox
finding: 'FieldShell.vue: the checkbox row took .field-label-row''s flex-wrap, so a label wider than the space beside the checkbox dropped to its own line, and a long label pushed the info button to the next line.'
severity: significant
resolution: 'FieldLabelRow.vue replaces FieldHint: a nowrap label row with the button, and the panel rendered after the row. Checked in the browser with a two-line checkbox label: the text wraps beside the box and the panel opens below.'
status: addressed
---
