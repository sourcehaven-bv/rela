---
id: RR-UUYDL10
type: review-response
title: Case-only rename strands successors on sqlite and pg
finding: retireID returns on EqualFold(old,new) before re-pointing rows whose successor is old. After A->B then B->b the row A:B stays. Successors(A) returns B, which no longer exists, and MentionSources(b) misses A's mentioners.
severity: significant
resolution: retireID re-points successors before the case-only early return on both database backends; storetest CaseOnlyRenameRepointsSuccessors pins it.
status: addressed
---
