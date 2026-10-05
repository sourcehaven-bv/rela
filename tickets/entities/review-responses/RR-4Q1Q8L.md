---
id: RR-4Q1Q8L
type: review-response
title: State-ref id could hint toward DeleteEntityFace
finding: A caller passing POL-1@draft to DeleteEntity could get a hint that DeleteEntityFace deletes one face.
severity: nit
reason: Error text and id parsing are unchanged by this fix; the HTTP surface already routes @face to the single-face delete. Not needed to close the defect.
status: wont-fix
---
