---
id: RR-L5VSYZ
type: review-response
title: Concurrent post/reload test never changes the shape
finding: TestService_ConcurrentPostReadReload reloads identical shapes and bypasses CheckReload/RecordTypes.
severity: nit
reason: It exists for the race detector; TestRecord_ConfigChangedDuringWrite covers the shape-change behaviour deterministically.
status: wont-fix
---
