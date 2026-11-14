---
id: RR-DOWIGL8
type: review-response
title: Docs imply removing read tools limits reads
finding: Entity and relation resources and the prompts still read whatever the ACL grants when show_entity and friends are filtered.
severity: significant
resolution: GUIDE-mcp-server, GUIDE-server-security and the WithToolFilter godoc say the filter narrows tools, not read access; TestWithToolFilter_ResourcesStayReadable pins it.
status: addressed
---
