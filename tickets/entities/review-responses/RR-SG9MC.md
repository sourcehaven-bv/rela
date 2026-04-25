---
id: RR-SG9MC
type: review-response
title: Guard runs after duplicate check; wrong error wins
finding: In workspace.createEntity, the duplicate-ID check runs before the IsManualID() guard in createEntityCore, so a caller passing an existing ID on a sequential/short type sees 'already exists' instead of the real 'you can't pass an ID for this type' error.
severity: significant
resolution: Moved the IsManualID() guard into the outer createEntity ahead of the duplicate lookup in internal/workspace/workspace.go. The inner createEntityCore still has the same guard as defense-in-depth. Extracted error construction into a shared customIDNotAllowedError helper so both sites produce identical messages.
status: addressed
---
