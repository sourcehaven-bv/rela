---
id: RR-PYZKMW
type: review-response
title: 'Q5: agree, signal by moving the session; scope MCP typing by backend'
finding: Moving into a working state is the only cross-process signal today, and the PoC's claim, approve and answer already do it. An MCP typing tool should wait for a cross-process channel path, and the plan should note that TKT-YK30D1 provides one only on postgres. For fs and SQLite, a stored activity entry with a TTL, or the fs channel directory watched by rela-server, would be needed.
severity: minor
resolution: 'Agreed: Q5 and the cross-process section say TKT-YK30D1 covers postgres only, and that fs and SQLite would need a stored activity entry with a TTL or a watched channel directory.'
status: addressed
---
