---
id: RR-AKGMZ
type: review-response
title: entityLabel inconsistent empty-string handling
finding: 'CommandPaletteModal.vue:130-135. `if (entity._title)` treats explicit empty-string _title as missing (truthy check). Line 133 explicitly checks `t !== ''''` for properties.title. Inconsistent. Fix: use the same check on both lines: `if (entity._title && entity._title !== '''')` or just `if (typeof entity._title === ''string'' && entity._title !== '''')`.'
severity: minor
resolution: Aligned both branches of entityLabel to use `typeof x === 'string' && x !== ''` (was just truthy check on _title). Empty-string _title now falls through to properties.title instead of being treated as a valid title.
status: addressed
---
