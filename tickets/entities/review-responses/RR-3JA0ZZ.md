---
id: RR-3JA0ZZ
type: review-response
title: Index error left the chunk undrained, restoring unbounded memory growth
finding: In backfillBleve, once flush() set indexErr it returned early, but the read loop kept running to the end of the corpus. Every remaining entity was appended to chunk and counted, and chunk was never drained again (clear/reslice sat after the early return), so it grew without bound holding every post-error entity alive. A corrupt index at entity 50 of 2389 would reproduce the ~1.1GB behaviour this ticket exists to remove. The 'skipped' count also conflated entities bleve rejected with entities never attempted.
severity: critical
resolution: The loop now breaks on the first index error instead of reading on, and the trailing flush is skipped when indexErr is set, so the chunk can never grow past backfillChunkSize. The skipped count is documented as 'read but not indexed' and derived from a total that only counts entities actually yielded. Pinned by TestBackfillBleve_StopsReadingAfterIndexError, which fails (reads 1000 of 1000 entities) if the break is removed.
status: addressed
---
