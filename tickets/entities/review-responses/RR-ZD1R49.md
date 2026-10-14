---
id: RR-ZD1R49
type: review-response
title: Go test overrides the draft-hiding default world
finding: TestPiles_WriteThroughWorldMiddleware called app.setWorlds(stubWorlds{...}) which leaves policy unscoped, so POL-1@draft was visible anyway and the test could not catch a named face resolved through the world chain.
severity: significant
resolution: Removed the setWorlds override; the test now runs against newPilesApp's draft-hiding default world.
status: addressed
---
