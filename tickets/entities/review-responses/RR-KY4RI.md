---
id: RR-KY4RI
type: review-response
title: Plan doesn't mention where InlineCreateModal is invoked from — scope of visual testing is unclear
finding: InlineCreateModal is referenced by 7 files (RelationPicker, tickets templates, etc.). The plan should note the main invocation path (RelationPicker.vue) and confirm that the prefix picker UX doesn't conflict with the 'add a new relation target inline' flow. In particular, for relation pickers, the caller may already know the target type — does the prefix picker make sense contextually? Probably yes (user still chooses a prefix when creating), but the plan should acknowledge it.
severity: nit
resolution: Plan's Research section now explicitly confirms InlineCreateModal is invoked from RelationPicker.vue; picker UX applies uniformly in both contexts.
status: addressed
---
