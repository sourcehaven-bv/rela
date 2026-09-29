---
id: RR-POJSNL
type: review-response
title: Batch skipped the address grammar check
finding: The old neighbor went through ParseRef; refIDs only rejected empty ids, so a malformed stored endpoint could be served.
severity: minor
resolution: ResolveHeaders drops refs that do not round-trip through entity.ParseRef (wellFormed). Pinned by TestResolver_ResolveHeadersRefusesMalformedRefs.
status: addressed
---
