---
id: RR-NGY84F
type: review-response
title: 'cards/list rows don''t pass attachments; `widget: file` would break there'
finding: 'PLAN-DMQFRJ Edge Cases claimed `widget: file` is ''legal, but the display arm needs the attachments prop, already passed''. Only true at ONE of the three SectionEditForm mount sites. Entry section (EntityDetail.vue:893) passes :attachments="entry._attachments". The cards row site (:982-992) and list row site (:1038-1048) pass no attachments at all. SectionEditForm.vue:300 and :315 read props.attachments?.[row.field.property], which is undefined on rows. Today this is harmless because defaultWidgetFor only selects ''file'' for a file-typed property and file properties don''t appear in card rows -- but once `widget: file` is authorable, an operator can force FileWidget into a card row and it renders with undefined attachments. This is exactly the failure mode buildSectionFieldData''s godoc (sections.go:73-79) warns about for the Go side: the Go construction sites were unified, the Vue mount sites were not.'
severity: significant
resolution: 'Verified all three mount sites in EntityDetail.vue -- confirmed, only the entry site passes attachments. Chosen resolution (a): reject `widget: file` outside a `properties`-display section at config load, which is in scope for this ticket''s size and fails loudly rather than rendering a broken widget. Plumbing _attachments onto ViewEntity for rows is the fuller fix and is deferred to a follow-up ticket. Documented explicitly in the plan rather than left silent.'
status: addressed
---
