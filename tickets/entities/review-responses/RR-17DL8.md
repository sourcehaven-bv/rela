---
id: RR-17DL8
type: review-response
title: 'isRrue ? help : undefined is a no-op conditional'
finding: 'FieldRenderer passes :help=''isRrue ? help : undefined'' to the widget. Non-rrule widgets ignore help anyway, so the conditional suppresses nothing observable. Either suppress FieldShell.help when rrule (intended fix to double-render?), or just always pass help (cleaner; same output).'
severity: minor
resolution: 'FieldRenderer.vue: dropped the isRrule computed and the conditional :help binding. Help is now always passed to the widget; non-rrule widgets ignore it (it''s already in their props interface and unrendered). Same observable behaviour, cleaner code.'
status: addressed
---
