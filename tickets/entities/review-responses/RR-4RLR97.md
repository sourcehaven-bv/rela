---
id: RR-4RLR97
type: review-response
title: Rich error status not flagged as a tool error
finding: A script returning status >= 400 produced a successful tool result.
severity: minor
resolution: run_action sets isError for a rich response with status >= 400 and says so in its description; test added.
status: addressed
---
