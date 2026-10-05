---
id: RR-YMFO8M
type: review-response
title: Zero-face guard advice contradicts the direct-read guard
finding: The zeroface failure message recommends store.GetEntityState and store.GetEntityAt. The new 8.5 guard forbids both in dataentry and mcp and lua.
severity: minor
resolution: The advice now leads with the resolver and visibility readers. It names the store reads only for code outside dataentry and mcp and lua.
status: addressed
---
