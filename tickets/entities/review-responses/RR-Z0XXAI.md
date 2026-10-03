---
id: RR-Z0XXAI
type: review-response
title: MCP relation resource does not percent-decode from
finding: A client expanding rela://relation/{from}/{type}/{to} sends POL-1%40draft, which ParseRef rejects.
severity: minor
resolution: The from segment goes through url.PathUnescape before ParseRef. Test case added.
status: addressed
---
