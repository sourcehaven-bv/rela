---
id: RR-WCXK13
type: review-response
title: No test pinned the denied-world + explicit-address case
finding: Two doc comments claimed the world grant cannot be bypassed by spelling the face, but nothing exercised a denied world with an ID@face address on the entity route or the entity view.
severity: significant
resolution: 'TestFacedAddress_DeniedWorldStillBlocks goes through the real router as a principal with no world grant: the entity route answers 404 and the entity view answers the world-absent page over the bare face, never the addressed face''s content.'
status: addressed
---
