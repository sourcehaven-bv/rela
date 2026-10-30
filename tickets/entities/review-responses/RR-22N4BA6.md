---
id: RR-22N4BA6
type: review-response
title: Shared e2e bug form now has a side panel
finding: Every spec using the bug form gets a side panel.
severity: nit
reason: Only task-item-conversions.spec.ts uses the bug form and it passes; there are no screenshot specs. A separate form would change the forms count asserted in settings.spec.ts.
status: wont-fix
---
