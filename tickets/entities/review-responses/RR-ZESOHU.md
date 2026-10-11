---
id: RR-ZESOHU
type: review-response
title: Mixed-version pg writers leave index stale
finding: Older binaries write bodies without updating the index; a one-time backfill never revisits them.
severity: significant
resolution: A BEFORE UPDATE trigger clears mention_targets when content changes but mentions_hash does not, which marks a row written by an older binary as not indexed. Such a row is not listed until the reindexer refills it (at open and every five minutes), so a backlink may be missing briefly but is never stale. Documented in postgres-backend.md.
status: addressed
---
