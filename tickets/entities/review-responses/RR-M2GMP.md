---
id: RR-M2GMP
type: review-response
title: .rela/lua-cache/ creation not in the plan
finding: The plan says disk path comes from `r.deps.ProjectRoot + "/.rela/lua-cache"` but the `writeToDisk` helper doesn't mention `MkdirAll`, and the test plan doesn't cover first-write-with-missing-dir.
severity: significant
resolution: 'Resolved by scope change: no disk in v1, so no directory to create. When v2 adds disk, `MkdirAll` on first write will be an explicit AC.'
status: addressed
---
