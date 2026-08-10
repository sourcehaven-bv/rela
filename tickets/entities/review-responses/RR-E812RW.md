---
id: RR-E812RW
type: review-response
title: updateCore godoc overstated "one read" as an end-to-end property
finding: The updateCore doc comment claimed threading oldEntity through 'keeps the write path at one read'. True inside the manager, false end-to-end for the MCP path, which still does its own GetEntity to obtain e.Type for validatePropertyNames before dispatching. Comments that claim a global property from a local vantage point age badly — and this one is adjacent to a ticket whose stated goal was consolidating duplicated reads.
severity: nit
resolution: 'Reworded to claim only what is true from where it sits: ''the manager itself reads the row once per write'', with an explicit note that callers may still read separately for their own reasons and naming internal/mcp as the case in point.'
status: addressed
---
