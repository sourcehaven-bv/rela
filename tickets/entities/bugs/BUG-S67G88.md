---
id: BUG-S67G88
type: bug
title: Authored-span property rows overlap and do not edit inline
description: 'On a view properties section with span: 4 / span: 2 fields (atlas taak view, Taakgegevens), values overlap the next field''s label and the Status value is hidden. Inline editing of those fields does not work; the body section does.'
priority: high
effort: s
why1: 'Each field is an RlDetailField with a fixed 120px label beside the value. At 1100px a span: 4 cell is 242px (value gets 106px) and the span: 2 Status cell is 109px (value gets 0px), so badges clip or overflow and Status disappears.'
why2: 'RlDetailField adapts its row only to the viewport (@media max-width: 767px). Inside the property grid its width comes from the authored span, not the viewport, so it never adapts.'
why3: TKT-ME8LEI moved SectionEditForm from stacked .property-item cells (label above value, the shape TKT-5V8704 spans were designed for) to side-by-side RlDetailField rows, but kept the span grid.
why4: No story or test renders RlDetailField in a narrow cell, and the migration was checked on views without spans, where every field is full width.
why5: 'Systemic: library components size themselves against the viewport rather than the box they are placed in, so a consumer that gives them a narrower box breaks without any signal.'
prevention: RlDetailField now lays out against its own width (wrapping flex row) instead of the viewport, and the e2e suite measures values in narrow span cells (AM-detail-field-narrow-span). Library layout components should size against their container, not the viewport.
started: "2026-10-01"
completed: "2026-10-01"
status: done
---

## Report

Atlas `views.taak` section "Taakgegevens" (`display: properties`, fields with
`span: 10/2/4/4/4`):

- Tag values (Bijdrage, Slaagkans, Werkinschatting) spill out of their cell and overlap the next field label.
- The Status value (`span: 2`, `render: input`) is not visible.
- Inline editing of these fields does not work. Inline editing of the body does.
