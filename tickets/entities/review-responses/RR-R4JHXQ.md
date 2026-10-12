---
id: RR-R4JHXQ
type: review-response
title: principal_property not re-resolved per pass
finding: 'Security and cranky review: Rebind resolves roles for the already-resolved principal, so a change to the user-entity mapping does not apply mid-watch, unlike live views. The docs implied parity.'
severity: minor
resolution: 'Documented in docs/acl-security.md: the mapping applies from the next call. Re-resolving would need the raw principal in the MCP ctx; the window is one call (at most 50 s) and needs an operator action.'
status: addressed
---
