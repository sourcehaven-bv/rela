---
id: RR-E5Q9C8
type: review-response
title: 'The vi.mock @/api comment stated a mechanism that cannot occur in this file'
finding: 'loadTemplates runs only in the create branch (DynamicForm.vue:1321) and this file mounts edit mode exclusively, so getTemplates is unreachable and the mock prevents nothing here. An ablation confirmed it: with the api mock alone and no SidePanel stub the request count was unchanged from baseline. Asserting it as the reason for the change is worse than no comment - the next reader treats it as load-bearing.'
severity: minor
resolution: 'Comment rewritten to state honestly that it is defensive symmetry with the sibling create-mode suites and not load-bearing here. Mock kept so a future create-mode case fails at the mock rather than at the adapter.'
status: addressed
---
