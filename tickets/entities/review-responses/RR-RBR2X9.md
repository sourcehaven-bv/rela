---
id: RR-RBR2X9
type: review-response
title: Withholding the id leaves restricted principals behind
finding: A principal who sees few types rarely gets an id-carrying frame, so its token goes stale.
severity: significant
resolution: 'Ping every 25 s carries id: Token(flushed). With per-type state an old token never forces a resync anyway.'
status: addressed
---
