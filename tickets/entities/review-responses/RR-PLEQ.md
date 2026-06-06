---
id: RR-PLEQ
type: review-response
title: Cache-Control on denied GET is weaker than existing noCacheMiddleware — reconcile
finding: 'noCacheMiddleware already sets ''Cache-Control: no-cache, no-store, must-revalidate'' on every /api/* route. Plan specifies ''private, no-store'' on denied GET — strictly weaker (no no-cache, no must-revalidate). Either match the existing middleware header (preferred — one less drift point: just don''t emit Cache-Control on the deny path and let the middleware win) OR document why the deny path needs different semantics. Also AC5 should explicitly assert response has NO ETag header (absence test) — catches a future regression where the handler computes ETag for the 404 path ''because it''s free.'''
severity: minor
status: open
---
