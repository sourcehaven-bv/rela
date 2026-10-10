---
id: RR-ALM2WD
type: review-response
title: ErrNotFound continue skips the second dropThreads
finding: When DeleteFamily reports ErrNotFound the post-delete drop is skipped.
severity: nit
reason: A concurrent delete through the entitymanager already fires the hook-based cleanup for that id.
status: wont-fix
---
