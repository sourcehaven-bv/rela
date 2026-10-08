---
id: RR-AOZU1S
type: review-response
title: Neutral denial depended on edge order; I/O errors became 403
finding: The first denial in store order won, so a hidden edge sorting before a visible one changed the answer. Any check error over a hidden edge was turned into a ForbiddenError and not logged.
severity: significant
resolution: All edges are judged; a visible denial wins over a hidden one. The subject decision is cached so equal edges get equal answers. Only forbidden errors become the neutral 403; other errors are logged and reported as errRelationCheck. Pinned by TestCascadeDelete_VisibleDenialWinsOverHidden.
status: addressed
---
