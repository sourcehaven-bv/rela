---
id: RR-3Z0W61E
type: review-response
title: History restore writes with UpdateEntity, outside the recheck
finding: 'pr-guidance G-005: restoreOntoLive wrote the restored entity with UpdateEntity, which has no in-Tx flow recheck. Batch was named too.'
severity: significant
resolution: restoreOntoLive now authorizes with AuthorizeUpdate and writes the gated set through PatchGranted, so it gets the in-Tx recheck, and properties it does not name stay as stored. Batch is create-only; a create has no run, so there is nothing to recheck. Conflict resolution writes the file directly on the fs tier only and is covered by TKT-LU8PSKJ.
status: addressed
---
