---
id: RR-VZTLY8
type: review-response
title: Hidden relation source still reached the ACL with its type
finding: The source type fed the face check and the ACL before the endpoint gate; a hidden source got not-found where a missing one got 403
severity: critical
resolution: lookupSource gates the source before the face check and the ACL and returns the zero family for a hidden source; TestWriteRequiresRead_DeclarativeHiddenSource compares both answers under a real policy
status: addressed
---
