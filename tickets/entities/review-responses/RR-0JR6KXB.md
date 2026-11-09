---
id: RR-0JR6KXB
type: review-response
title: 'Design: peers inherit the wiring-level verified-JWT flag'
finding: Opener marks every identity verified when the JWT gate is installed; a peer would count as unmatched-verified in JWT mode.
severity: minor
resolution: Peers cannot reach a write route, so provisioning (write path only) never runs for them, and provision refuses reserved names anyway. Documented in the edge comment.
status: addressed
---
