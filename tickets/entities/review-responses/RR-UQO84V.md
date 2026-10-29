---
id: RR-UQO84V
type: review-response
title: Plan misreads TKT-S5OJ7S as the home for authored button label/order/style/confirm
finding: 'Scope (out), Alternatives and Open question 6/10 say explicit label, order, style and confirm authoring ''belongs to TKT-S5OJ7S (session.offer:)'' and that decisionViews is where ''an offer list filters, orders and styles the buttons''. TKT-S5OJ7S is something else: `session.offer: transitions` fills MessageType.In from on: at load, so a transition type is postable only where it applies. It has no per-transition label, style or confirm. Consequences: (1) no ticket owns an authored confirm/label/style override, so the ''derive confirm from final'' rule has no planned escape hatch; open a follow-up or drop the claim. (2) S5OJ7S is in fact the server-side mitigation for this plan''s stray-post risk (a late click from a second tab is refused with 409 instead of stored as a no-op), and the PoC realtime.yaml declares no in: at all. Name S5OJ7S as the mitigation in Risks and docs, and consider sequencing it first or together.'
severity: significant
resolution: 'Corrected the TKT-S5OJ7S reading: it is now named in Scope, Risks, Open questions 4 and docs as the server guard against a no-op late post and recommended next, and a new follow-up "Authored confirm and style for session decisions" owns the confirm/style override.'
status: addressed
---
