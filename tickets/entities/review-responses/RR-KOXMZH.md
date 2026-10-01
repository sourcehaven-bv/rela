---
id: RR-KOXMZH
type: review-response
title: GraphQuery with OrderBy/Offset and no Limit loads everything
finding: The single-statement branch buffers the whole result when a caller sorts or offsets without a Limit.
severity: minor
resolution: Documented on graphPages; no caller does this today.
status: addressed
---
