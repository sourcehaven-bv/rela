---
id: RR-C45OR
type: review-response
title: streamToFile writes directly (no temp+rename); crash mid-copy corrupts attachment
finding: OpenForWrite used by streamToFile opens the final key directly. Mid-copy crash leaves a truncated file at the final path; in-memory index then records the truncated size as 'real.' No fsync either.
severity: significant
reason: Pre-existing behavior — old streamToFile also wrote directly with no temp+rename (verified against origin/develop). Not a regression introduced by this PR. Fixing properly requires either an AtomicWriter type on RootedFS.OpenForWrite or caller-side temp+rename logic. Tracked separately; see TKT-TX53E (read-path migration) which will revisit attachment durability as part of the attachment-Remove cleanup.
status: deferred
---
