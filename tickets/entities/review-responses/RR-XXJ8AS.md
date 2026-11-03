---
id: RR-XXJ8AS
type: review-response
title: Mention text in AC 3's uniform-answer test cannot be byte-identical across the four cases
finding: 'AC 3 says the answer is the same whether the id is a hidden user, a missing entity, a non-user, or a user who cannot read the channel. The fourth case is a user the poster CAN read, so the answer''s `mentions` lists it as a user chip and the other three do not. TestPost_MentionAnswerIsUniform as described would fail, or would be weakened until it passes. Fix: restate it as two equalities. Hidden user, missing entity and non-user give equal answers. A readable user who can read the channel and one who cannot give equal answers.'
severity: minor
resolution: AC 3 restated as two equalities (hidden/missing/non-user equal; readable user with or without channel access equal).
status: addressed
---
