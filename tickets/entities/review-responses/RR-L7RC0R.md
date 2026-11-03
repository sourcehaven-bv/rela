---
id: RR-L7RC0R
type: review-response
title: Hidden-field refusal audit omits the source
finding: '[security] The denied-write row for a copied hidden field records only the target type subject (taskitemconversion.go visibleTaskItemSource), not the source entity, so an auditor cannot see which sources were probed.'
severity: minor
resolution: The hidden-field denial now records the source entity as the audit subject, and the reason names the target type. Pinned in TestTaskItemConvert_RefusesToCopyAnUnreadableField, which asserts the denied-write subjects are the two sources.
status: addressed
---
