---
id: RR-YDSA0
type: review-response
title: 'display: properties on non-entry source is dead code (latent issue, out of scope)'
finding: buildSections (sections.go:183) lumps 'properties' and 'list' together for non-entry sources, populating sd.Entities. Frontend renders 'properties' via PropertyDisplay which only reads section.fields, ignoring section.entities. Dead path. Not in scope but flagged for awareness so we don't accidentally extend the bug.
severity: significant
reason: 'Pre-existing latent dead-code path (display: properties on non-entry source). Surfaced during review, but unrelated to the bug being fixed. Not blocking; will file a separate cleanup ticket if it actually causes confusion.'
status: deferred
---
