---
id: RR-XOLFQ6
type: review-response
title: Visible overwrote a caller-supplied Query.Admit
finding: visibleHits replaced q.Admit with the scope's admission.
severity: nit
resolution: A caller's Admit now runs on what the scope admitted. Pinned by TestVisible_CallerAdmitNarrowsTheScope; Query.Admit doc updated.
status: addressed
---
