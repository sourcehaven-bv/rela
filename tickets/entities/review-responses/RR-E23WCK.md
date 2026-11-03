---
id: RR-E23WCK
type: review-response
title: SQL plan claims are verified by hand, not pinned
finding: '"Verified on sqlite (system build) and postgres 18" does not cover the modernc.org/sqlite driver the binary uses, and nothing pins the planner''s choices. Fix: add EXPLAIN QUERY PLAN tests in sqlitechannels (due_explain_test.go pattern) and a pgstore explain test. They pin that the count subquery uses channel_messages_counts_idx (the literal `counts = 1` matches the partial index), that Reads is one statement per 500-key chunk, and that ListReads uses channel_reads_reader_idx.'
severity: minor
resolution: Added reads_explain_test.go (sqlitechannels, modernc) and channel_reads_explain_test.go (pgstore) pinning counts_idx, one statement per 500-key chunk, and reader_idx for ListReads.
status: addressed
---
