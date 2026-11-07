---
id: RR-HO8TD8W
type: review-response
title: Small doc and ETag nits
finding: (a) Limbs doc said PostgreSQL bigint holds the sum; SUM(bigint) returns numeric. (b) The same-function fact was repeated in four comments. (c) The relation ETag was unquoted hex. (d) FormatMarkdown now runs per single-entity ETag. (e) Faces of one family are not adjacent in bytewise order.
severity: nit
resolution: (a) Doc now says store bigint and cast the sum. (b) Kept on canonical.Digest and cited elsewhere. (c) The relation ETag now uses the shared quoted viewETag form. (d) Documented on servedETag as single-entity only. (e) Accepted design; noted for later steps.
status: addressed
---
