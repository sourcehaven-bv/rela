---
id: RR-DZXT0ZY
type: review-response
title: 'Design: missed write paths'
finding: 'The cascade host, MCP, LLM apply, caldav and importer paths were not accounted for.'
severity: significant
resolution: 'MCP, actions and other transports go through Manager. The cascade host runs the check and refuses any out-of-scope write, including one that would need a clear (an automation does not clear). Copy and conversion are listed as out of scope with a follow-up ticket. A clear that fires a trigger which creates an edge enters the check again, as every create does.'
status: addressed
---
