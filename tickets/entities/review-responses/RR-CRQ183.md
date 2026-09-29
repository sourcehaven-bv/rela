---
id: RR-CRQ183
type: review-response
title: Last-face DeleteFace leaves inbound edges without a stated contract
finding: Section 2.4 says incoming edges survive DeleteFace and that deleting the last face leaves no entity. It does not say what happens to inbound edges then or that storetest pins it.
severity: minor
resolution: 'Amendment A3: documented as identical to DeleteFamily(id false); cascade stays the manager''s decision; storetest pins it in PR 6.'
status: addressed
---

## Finding

Section 2.4 says incoming edges survive DeleteFace and that deleting the last
face leaves no entity. It does not say what happens to inbound edges then or
that storetest pins it.

Design: `.ignored/stage2-design.md` section 11.
