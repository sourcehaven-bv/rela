---
id: RR-BTUN4L
type: review-response
title: Badge entityType disambiguator not wired at widget/detail call sites
finding: Only EntityList/KanbanView pass :entity-type to Badge. SelectWidget/MultiSelectWidget receive entityType in WidgetProps but don't forward it, and EntityDetail's four Badge cells pass nothing — so same-named properties backed by different custom types fall to the arbitrary first-wins tie-break.
severity: significant
resolution: 'SelectWidget and MultiSelectWidget now forward :entity-type="entityType" to Badge. EntityDetail was investigated and deliberately NOT wired: its view-section cells pass cell.propType — the property''s TYPE name (sections.go populates PropType from pd.Type) — as :property, which is already the exact styles key; the store''s direct-key fallback resolves it, now documented in the stylesForProperty comment and pinned by a schema.test.ts case.'
status: addressed
---
