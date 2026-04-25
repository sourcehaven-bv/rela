---
id: RR-5K8I2
type: review-response
title: DynamicForm composable wrapping is square-peg
finding: 'DynamicForm does NOT render a Back button; it snapshots return_to at mount and router.push()es on submit/cancel. Composable returns reactive {to, label} oriented at rendering. Wrapping one in the other gives a vestigial label the form ignores and reactivity it doesn''t need. The existing readReturnTo primitive in returnPath.ts already covers the shared bit. Options: (a) DynamicForm keeps calling readReturnTo directly (composable also uses it); (b) drop AC6''s ''route through the composable'' from scope. Don''t invent indirection.'
severity: minor
resolution: 'Dropped DynamicForm refactor from scope (scope item #7). Form keeps calling readReturnTo directly; that''s the shared primitive between form and composable. No indirection invented.'
status: addressed
---
