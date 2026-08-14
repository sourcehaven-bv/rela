---
id: RR-CR-SLOTUNUSED
type: review-response
title: '<rela-slot> is documented as a tier-1 contract but the SPA emits it nowhere'
finding: "docs/customisation.md promises 'Tier 1 — Real contract: tag name, attributes in, events out. A break is a rela bug.' but grep finds zero occurrences of <rela-slot> in frontend/src. Only isCustomElement ships, which merely stops Vue warning about a tag that does not exist. An operator following the docs would define the element and see nothing render."
severity: minor
status: addressed
resolution: 'Docs corrected to state plainly that no slot is emitted yet and that the tag is reserved, with a pointer that the first consumer arrives with the next-action feature (items 4-5, explicitly out of scope for this ticket and blocked on that component). The isCustomElement config is still correct and necessary today: it is what makes the tag inert rather than warning, which is the precondition for shipping a slot later.'
---

Raised by `/code-review` of the TKT-3DBK6I implementation.
