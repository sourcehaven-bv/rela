---
id: RR-A26AZ
type: review-response
title: backTargetAfterDelete falls back to / in projects without dashboard
finding: EntityDetail.vue:253-258 returns '/' as last-resort after delete. In minimal rela deployments without a configured dashboard, '/' is the SPA entrypoint that then router-redirects somewhere undefined. Not a regression — behavior preserved from the prior scopeNav.backUrl chain — but worth a TODO or a more defensive default (e.g., '/analyze' which always exists).
severity: minor
reason: Out of scope for TKT-JIEKC (the reviewer noted it's a pre-existing behavior preserved from the prior scopeNav.backUrl chain). '/' is always a valid router target — the SPA redirects '/' to '/dashboard' in the router config. 'Dashboard has no cards' is a visually sparse state, not a broken one. If a minimal-deployment UX concern surfaces later, file a dedicated ticket.
status: wont-fix
---
