---
id: RR-L6XZSZ
type: review-response
title: The audit loses the poster's original tool
finding: 'The inline write copies the request principal and replaces Tool with `realtime`. ACL decisions do not read Tool (only isUnstamped does), so this is safe, but the audit can no longer tell an MCP agent''s output from a data-entry user''s. Plan change: put the original tool into triggered_by, or into the channel audit record of the done write.'
severity: nit
resolution: The mark's Writer keeps the cause author's original tool, shown on the wire and in the channel done audit record. The graph audit keeps tool realtime and a triggered_by naming the move.
status: addressed
---
