---
id: RR-EA0URF
type: review-response
title: Fs comment store loses writes across processes
finding: A second process (stdio rela mcp --run-as) now writes comments. filecomments only locked with an in-process mutex, so concurrent adds from rela-server and rela mcp could drop a comment silently.
severity: significant
resolution: filecomments takes an injected cross-process Locker (WithLocker) on every write; appbuild passes filelock on cache/comments.lock (process-private for MemFS). TestAdd_TwoStoresLoseNoComment fails without it (22 of 40 kept).
status: addressed
---
