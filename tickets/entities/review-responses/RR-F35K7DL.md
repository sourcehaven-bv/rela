---
id: RR-F35K7DL
type: review-response
title: 'Section-level render: input on tables is an existing inert key'
finding: inertSectionRenderWarnings warns about it; making it apply would make existing configs editable on upgrade. Keep editability column-level.
severity: minor
resolution: 'Editability is per column; a section''s render: does not apply to table columns. Documented in the guide.'
status: addressed
---
