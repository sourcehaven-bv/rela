---
id: RR-4ND3IST
type: review-response
title: No e2e coverage
finding: No e2e test covers a portal list without checkboxes.
severity: nit
reason: The selection rule is a computed covered by component tests, and the per-principal value by the handler test; page-chrome.spec.ts already covers the same toggle pipeline end to end.
status: wont-fix
---
