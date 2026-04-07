---
id: RR-U82Z
type: review-response
title: Curl/non-browser clients are blocked by default — needs documentation
finding: The reviewer noted that requireSameOrigin rejects requests with no Origin and no Referer. This intentionally catches no-referrer attacks but also blocks legitimate non-browser clients (curl, scripts, MCP integrations), and was not documented anywhere.
severity: minor
resolution: 'Added a ''Calling the API from curl, scripts, or non-browser clients'' section to docs/security.md with the blessed `curl -H ''Origin: ...''` pattern. Also added a Troubleshooting section that explains the three rejection reasons (host_not_allowed, origin_not_allowed, origin_missing) so operators can self-diagnose 403s.'
status: addressed
---
