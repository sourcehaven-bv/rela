---
id: RR-0IJ0DB
type: review-response
title: Wrong package path and plimsoll budget
finding: v1.Position lives in internal/apiwire/v1; App is at its plimsoll cap; migration numbers may move.
severity: nit
resolution: 'Plan: wire types in internal/apiwire/v1; handlers on a pilesHandler struct like commentsHandler; SetPiles is the only new App method (within the exported cap, checked by just plimsoll). Re-check pg migration number at merge.'
status: addressed
---
