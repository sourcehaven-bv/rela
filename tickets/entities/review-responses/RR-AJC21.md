---
id: RR-AJC21
type: review-response
title: 'Log-noise risk: warning lines in document body'
finding: rela.output warning appears in rendered HTML. A script using rela.output inside a loop could emit many warnings that dominate the visible doc. Action-mode has the same caveat. Fine to defer; worth noting in guide.
severity: nit
resolution: Log-noise caveat for rela.output in loops added to AC-DOC1 guide requirements.
status: addressed
---

From design-review on PLAN-78HJO. Deferrable; mention in GUIDE-data-entry under
the caveats subsection.
