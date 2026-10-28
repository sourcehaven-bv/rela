---
id: RR-XLFCDS
type: review-response
title: Missing test cases for write 404 parity
finding: Only reads checked the uniform 404 for an existing and a missing id.
severity: nit
resolution: Added update and delete cases for TKT-001 and TKT-404 with nothing bound. The suggested list search case answers 200 in this fixture, so it was not added.
status: addressed
---
