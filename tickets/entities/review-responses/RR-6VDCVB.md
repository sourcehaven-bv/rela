---
id: RR-6VDCVB
type: review-response
title: Configure list page can write an invalid busy column
finding: 'The plan changes only models.ts (columnModel, columnTitle) for the Configure list page. frontend/src/views/configure/ListView.vue:148-153 shows a Sortable switch on every column and writes sortable through column(index, ''sortable'', value). On a busy column that switch writes sortable: true, which AC 1 refuses at load, so the UI offers an edit that the save always rejects. The same file shows item.property || item.relation as the placeholder and source text (ListView.vue:140,146), so a busy column shows a blank source. The preview passes c.sortable through as well (ListView.vue:109). changes.ts:500-507 builds its own ColumnModel literal for the change review and is not in the file list, so it fails to compile once ColumnModel gains busy, or titles the column ''column'' if busy is optional. Fix: hide the Sortable switch for busy (and face) columns, show the channel type as the source text, and add ListView.vue and changes.ts to the files and to AC 11 (a test that a busy row has no Sortable switch and shows ''run'').'
severity: significant
resolution: 'ADDRESSED ListView.vue and changes.ts added to step 13 and the file list; busy and face columns get no Sortable switch, source text and placeholder read busy: <type> via columnSource, preview marks them not sortable, changes.ts literal reads busy; AC 11 extended with models, changes and new ListView.test.ts cases'
status: addressed
---
