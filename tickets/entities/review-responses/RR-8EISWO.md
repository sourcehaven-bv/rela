---
id: RR-8EISWO
type: review-response
title: Section-gate change fixes a staleness bug the all-display case cannot have
finding: 'The plan''s risk-4 mitigation changes `sectionShouldRouteToInlineEdit` (sectionEditFields.ts:97-99) to ignore display-flagged fields, so an all-display section still mounts SectionEditForm — justified as avoiding PropertyDisplay''s stale stringified `values`. But the staleness requires an editable sibling field to make a value stale, and an all-display section has none by definition. The mitigation therefore delivers zero benefit while adding mounts and, more importantly, silently inverting heading ownership: `sectionRendersOwnHeading` (EntityDetail.vue:503-510) gates on `sectionShouldRouteToInlineEdit`, so after the change essentially every properties section would suppress the generic <h2> and render SectionEditForm''s own header row — including an AutoSaveIndicator on sections that can never save.'
severity: significant
resolution: Dropped the section-gate change entirely. `sectionShouldRouteToInlineEdit` stays untouched; all-display sections route to PropertyDisplay exactly as today. Heading ownership unchanged. AC 6 rewritten to assert observable behaviour rather than which component mounts.
status: addressed
---

Verified: `mapFieldsToProperties` (EntityDetail.vue:423-449) reads `field.values
?? []` (server data), never `entry.properties` — so the entry-section staleness
the plan cited is real. But it can only manifest when a sibling field in the
same section was edited, which requires a `render: input` field. A mixed
input+display section already routes to inline edit under the **unmodified**
predicate, because at least one field is writable-and-input.

The heading-ownership inversion is the decisive argument, not mount cost — it is
a visible behaviour change affecting every properties section by default.

Note on mount cost (raised in review as "up to 100 dead autosave hosts"): the
reviewer overstated this. `INLINE_EDIT_ROW_CAP = 100` (EntityDetail.vue:550) is
a soft cap for the pathological case, and its own comment says instances are
"cheap individually". Typical sections are 3-10 rows. Cost was not the reason to
drop the change.

The residual pre-existing staleness in `mapFieldsToProperties` for **mixed**
sections predates this ticket and is left as a separate concern; see RR-CJKDXG.
