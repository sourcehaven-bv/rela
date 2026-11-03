---
id: RR-PM6JWK
type: review-response
title: in_derived has no TS comment
finding: 'frontend/src/types/channels.ts:617-619: the comment covers in only; in_derived is bare, unlike decision_button and choices above it. Either one comment that names both fields ("in and in_derived: see ...") or a short one on in_derived.'
severity: nit
resolution: "addressed\tin_derived has its own TS comment: true when in was derived from session.default_in."
status: addressed
---
