---
id: RR-7ISAHX6
type: review-response
title: Error lines point at the alias target or the block
finding: refuseNullAllow reported the anchor line and the empty-allow error the remote_mcp line.
severity: nit
resolution: Errors report the line of the value node itself; the empty check moved to the allow node.
status: addressed
---
