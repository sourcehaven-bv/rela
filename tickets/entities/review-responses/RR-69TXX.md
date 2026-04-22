---
id: RR-69TXX
type: review-response
title: Windows case-sensitivity of root path may cause LRU key divergence
finding: On NTFS/APFS, NewRootedFS cleans but doesn't canonicalize case. If caller passes 'c:\Root' and fsnotify delivers 'C:\Root\...', LRU keys diverge.
severity: significant
reason: Speculative edge case with low likelihood (production callers always pass canonical paths from project.Discover). Out of scope for this PR. Documented in the RootedFS doc comment for future reference.
status: wont-fix
---
