---
id: RR-XVJT46
type: review-response
title: familyAny reads the same headers twice
finding: storedType read the headers and Resolver.Family read them again; every MCP readable call paid two header reads.
severity: minor
resolution: Family is split into headersOf and familyOf; familyAny reads the headers once and runs admit on the type found. Pinned by TestScriptReader_FamilyReadsHeadersOnce.
status: addressed
---
